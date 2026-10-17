package gamestats

class ValidationSuite extends munit.FunSuite:
  import TestData.*
  test("all approved profiles are recognized, while pending capabilities remain explicit") {
    val loaded = right(Profiles.load(profiles))
    assertEquals(loaded.keySet, Set("racing", "snake", "tic-tac-toe"))
    assertEquals(loaded("racing").pending, Vector.empty)
    assertEquals(loaded("snake").pending, Vector("temporal-window/v1"))
    assertEquals(loaded("tic-tac-toe").pending, Vector("tic-tac-toe-line/v1"))
  }
  val badProfiles: Vector[(String, Json => Json)] = Vector(
    "version" -> (j => update(j, "profileSchemaVersion", Json.IntNum(2))),
    "unknown property" -> (j => update(j, "extra", Json.Bool(true))),
    "malformed revision" -> (j => update(j, "profileVersion", Json.Str("latest"))),
    "unknown operation" -> (j => editMetric(j, 2)(m => update(m, "operation", Json.Str("min")))),
    "missing numeric field" -> (j => editMetric(j, 2)(m => remove(m, "field"))),
    "numeric type" -> (j => editMetric(j, 2)(m => update(m, "numberKind", Json.Str("text")))),
    "numeric minimum" -> (j => editMetric(j, 2)(m => update(m, "minimum", Json.Str("zero")))),
    "duplicate output" -> (j => editMetric(j, 1)(m => update(m, "id", Json.Str("laps.player")))),
    "count with field" -> (j => editMetric(j, 0)(m => update(m, "field", Json.Str("/data/lap")))),
    "wildcard pointer" -> (j => editMetric(j, 2)(m => update(m, "field", Json.Str("/data/*")))),
    "invalid pointer escape" -> (j => editMetric(j, 2)(m => update(m, "field", Json.Str("/data/~3")))),
    "unknown template version" -> (j =>
      val caps = array(field(j, "capabilities"))
      update(j, "capabilities", Json.Arr(caps.updated(0, update(caps.head, "templateId", Json.Str("event-metrics/v2")))))),
    "duplicate instance" -> (j =>
      val caps = array(field(j, "capabilities"))
      update(j, "capabilities", Json.Arr(caps.updated(1, update(caps(1), "instanceId", Json.Str("laps"))))))
  )
  badProfiles.foreach { (name, change) =>
    test("configuration rejects " + name) { assert(Decoding.profile(change(profile)).isLeft) }
  }
  test("two profiles for the same game fail before server startup") {
    assertEquals(Profiles.fromJson(Vector(profile, profile)).left.map(_.code), Left("duplicate_game"))
  }
  test("missing profile directory is not confused with an unknown game") {
    assertEquals(Profiles.load(profiles.resolve("missing")).left.map(_.code), Left("profile_directory"))
  }
  test("unknown analyzer and duplicate cross-template output IDs are rejected") {
    val board = file(profiles.resolve("tic-tac-toe.json"))
    val analyzers = array(field(board, "specializedAnalyzers"))
    val unknown = update(board, "specializedAnalyzers", Json.Arr(Vector(
      update(analyzers.head, "analyzerId", Json.Str("unknown/v1")))))
    assert(Decoding.profile(unknown).isLeft)
    val duplicated = update(board, "specializedAnalyzers", Json.Arr(Vector(
      update(analyzers.head, "parameters", update(field(analyzers.head, "parameters"),
        "resultMetricId", Json.Str("moves.player"))))))
    assert(Decoding.profile(duplicated).isLeft)
  }

  val badRequests: Vector[(String, Json => Json)] = Vector(
    "empty collection" -> (j => update(j, "events", Json.Arr(Vector.empty))),
    "invalid state" -> (j => update(j, "state", Json.Str("finished"))),
    "missing state" -> (j => remove(j, "state")),
    "mixed game" -> (j => editEvent(j, 1)(e => update(e, "gameId", Json.Str("other")))),
    "mixed match" -> (j => editEvent(j, 1)(e => update(e, "matchId", Json.Str("other")))),
    "duplicate event" -> (j => editEvent(j, 1)(e => update(e, "eventId", Json.Str("s")))),
    "missing start" -> (j => editEvent(j, 0)(e => update(e, "type", Json.Str("CUSTOM")))),
    "second start" -> (j => editEvent(j, 1)(e => update(e, "type", Json.Str("MATCH_STARTED")))),
    "open with finish" -> (j => update(j, "state", Json.Str("open"))),
    "closed without finish" -> (j => editEvent(j, 4)(e => update(e, "type", Json.Str("CUSTOM")))),
    "finish is not last" -> (j =>
      val events = array(field(j, "events"))
      update(j, "events", Json.Arr(events.updated(3, events(4)).updated(4, events(3))))),
    "wrong schema" -> (j => editEvent(j, 1)(e => update(e, "schemaVersion", Json.IntNum(2)))),
    "decimal schema" -> (j => editEvent(j, 1)(e => update(e, "schemaVersion", Json.DecNum(BigDecimal("1.0"))))),
    "invalid timestamp" -> (j => editEvent(j, 1)(e => update(e, "timestamp", Json.Str("2026-02-30T00:00:00Z")))),
    "timezone missing" -> (j => editEvent(j, 1)(e => update(e, "timestamp", Json.Str("2026-10-17T18:00:00")))),
    "empty required string" -> (j => editEvent(j, 1)(e => update(e, "eventId", Json.Str("  ")))),
    "optional null" -> (j => editEvent(j, 1)(e => update(e, "playerId", Json.Null))),
    "non-object data" -> (j => editEvent(j, 1)(e => update(e, "data", Json.Arr(Vector.empty))))
  )
  badRequests.foreach { (name, change) =>
    test("snapshot rejects " + name) {
      assert(Decoding.snapshot(change(fixture("race-final", "request"))).isLeft)
    }
  }
  Vector("", "{", "null true", "{} {}", """{"x":1,"x":2}""", """{"data":{"x":1,"x":2}}""",
    """{"number":1e1000000}""", "[" * 65 + "0" + "]" * 65).foreach { bad =>
    test("strict JSON rejects malformed/ambiguous/complex input: " + bad.take(35)) {
      assert(Json.parse(bad).isLeft)
    }
  }
  test("subnanosecond timestamp parsing matches Go truncation") {
    val request = editEvent(fixture("race-final", "request"), 4)(e =>
      update(e, "timestamp", Json.Str("2026-10-17T18:00:32.123456789123Z")))
    assertEquals(right(Decoding.snapshot(request)).events.last.timestamp.getNano, 123456789)
  }
  test("object pointer escapes work and arrays are not implicitly traversed") {
    val data = parse("""{"data":{"a/b":{"~x":42},"items":[5]}}""")
    assertEquals(Json.pointer(data, "/data/a~1b/~0x"), Some(Json.IntNum(42)))
    assertEquals(Json.pointer(data, "/data/items/0"), None)
  }
  test("server environment validation is explicit") {
    assertEquals(right(ServerConfig.fromEnv(Map.empty)).port, 9000)
    assert(ServerConfig.fromEnv(Map("SCALA_PORT" -> "zero")).isLeft)
    assert(ServerConfig.fromEnv(Map("SCALA_PORT" -> "65536")).isLeft)
    assert(ServerConfig.fromEnv(Map("SCALA_HOST" -> " ")).isLeft)
  }

  test("RFC3339 offsets up to 23:59 remain compatible with Go") {
    val request = editEvent(fixture("race-final", "request"), 4)(e =>
      update(e, "timestamp", Json.Str("2026-10-18T17:59:32+23:59")))
    assertEquals(right(Decoding.snapshot(request)).events.last.timestamp,
      snapshot("race-final").events.last.timestamp)
  }

  test("response encoding is bounded rather than allocating the complete oversized JSON") {
    val body = Json.Obj(Map("large" -> Json.Str("x" * 1024)))
    assertEquals(Json.boundedBytes(body, 100), None)
    assertEquals(Json.boundedBytes(body, 2048).map(parseBytes => right(Json.parse(parseBytes))),
      Some(body))
  }
