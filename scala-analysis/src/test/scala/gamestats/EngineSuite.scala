package gamestats

class EngineSuite extends munit.FunSuite:
  import TestData.*
  supported.foreach { id =>
    test("complete approved fixture: " + id) {
      val result = right(engine.analyze(snapshot(id)))
      assertEquals(result.json, fixture(id, "expected"))
      assertEquals(right(engine.analyze(snapshot(id))).json, result.json)
    }
  }

  test("a new game is only a profile change; composition is preserved") {
    val configured = update(profile, "gameId", Json.Str("new-game"))
    val request = fixture("race-final", "request")
    val renamed = update(update(request, "gameId", Json.Str("new-game")), "events",
      Json.Arr(array(field(request, "events")).map(e => update(e, "gameId", Json.Str("new-game")))))
    val result = analyze(renamed, configured)
    assertEquals(result.gameId, "new-game")
    assertEquals(result.metrics.size, 8)
    assertEquals(metric(result, "laps.player").value, Some(BigDecimal(3)))
    assertEquals(metric(result, "duration.match", "race-live").value, Some(BigDecimal(32)))
  }

  test("an added configured metric executes without engine changes") {
    val caps = array(field(profile, "capabilities"))
    val metrics = array(field(caps.head, "metrics"))
    val added = update(metrics.head, "id", Json.Str("extra.count"))
    val configured = update(profile, "capabilities", Json.Arr(caps.updated(0,
      update(caps.head, "metrics", Json.Arr(metrics :+ added)))))
    val result = analyze(fixture("race-final", "request"), configured)
    assertEquals(metric(result, "extra.count").value, Some(BigDecimal(3)))
    assertEquals(metric(result, "laps.player").value, Some(BigDecimal(3)))
  }

  Vector(Json.Str("100"), Json.Bool(true), Json.Null, Json.IntNum(-1), Json.DecNum(BigDecimal("1.5"))).foreach { bad =>
    test("invalid number is explicit, while counts remain valid: " + bad.toString) {
      val result = analyze(editData(fixture("race-live-2", "request"), 1, "durationMs", bad))
      assertEquals(metric(result, "laps.player").value, Some(BigDecimal(2)))
      Vector("lap_time.total.player", "lap_time.avg.player").foreach { id =>
        assertEquals(metric(result, id).status, "invalid")
        assertEquals(metric(result, id).reason, Some("invalid_number"))
        assertEquals(metric(result, id).value, None)
      }
    }
  }
  test("missing field never becomes zero or disappears from the denominator") {
    val request = editEvent(fixture("race-live-2", "request"), 1)(event =>
      update(event, "data", remove(field(event, "data"), "durationMs")))
    assertEquals(metric(analyze(request), "lap_time.avg.player").reason, Some("missing_field"))
  }

  test("exact sum exceeds DECIMAL128 precision without rounding") {
    val huge = BigInt("999999999999999999999999999999999999999999999")
    val request = editData(editData(fixture("race-live-2", "request"), 1,
      "durationMs", Json.IntNum(huge)), 2, "durationMs", Json.IntNum(1))
    val result = metric(analyze(request), "lap_time.total.player").value.get
    assertEquals(result.toBigInt, huge + 1)
  }
  test("decimal values and HALF_EVEN averages remain exact") {
    val configured = editMetric(editMetric(profile, 2)(m =>
      update(update(m, "numberKind", Json.Str("decimal")), "minimum", Json.IntNum(0))), 3)(m =>
      update(update(m, "numberKind", Json.Str("decimal")), "minimum", Json.IntNum(0)))
    val request = editData(editData(fixture("race-live-2", "request"), 1, "durationMs",
      Json.DecNum(BigDecimal("1.000000"))), 2, "durationMs", Json.DecNum(BigDecimal("1.000001")))
    assertEquals(metric(analyze(request, configured), "lap_time.avg.player").value,
      Some(BigDecimal("1.000000")))
    val next = editData(request, 1, "durationMs", Json.DecNum(BigDecimal("1.000002")))
    assertEquals(metric(analyze(next, configured), "lap_time.avg.player").value,
      Some(BigDecimal("1.000002")))
  }
  test("roster invalidity returns an explicit snapshot error") {
    val request = editData(fixture("race-final", "request"), 0, "players",
      Json.Arr(Vector(Json.Str("p1"), Json.Str("p1"))))
    assertEquals(engine.analyze(right(Decoding.snapshot(request))).left.map(_.code), Left("invalid_roster"))
  }
  test("missing/unknown player invalidates dependent player metrics, not match counts") {
    Vector(None, Some(Json.Str("outsider"))).foreach { player =>
      val request = editEvent(fixture("race-live-2", "request"), 1)(event =>
        player.fold(remove(event, "playerId"))(v => update(event, "playerId", v)))
      val result = analyze(request)
      assertEquals(metric(result, "laps.player").status, "invalid")
      assertEquals(metric(result, "laps.match", "race-live").value, Some(BigDecimal(2)))
    }
  }
  test("without a declared roster only observed players are emitted") {
    val configured = remove(profile, "playersField")
    val result = analyze(fixture("race-live-2", "request"), configured)
    assert(!result.metrics.exists(_.subjectId == "p2"))
    assertEquals(metric(result, "laps.player").value, Some(BigDecimal(2)))
  }

  private def lifetimeProfile: Json =
    // A separate test profile, not a rewrite of the approved Snake configuration.
    val caps = array(field(file(profiles.resolve("snake.json")), "capabilities"))
    update(update(profile, "capabilities", Json.Arr(Vector(caps(2)))), "gameId", Json.Str("snake"))

  test("pure start-to-death duration works without executing pending Snake window") {
    val result = analyze(fixture("snake-positive", "request"), lifetimeProfile)
    assertEquals(metric(result, "lifetime.player").value, Some(BigDecimal(15)))
    assertEquals(metric(result, "lifetime.player", "p2").value, Some(BigDecimal(18)))
  }
  test("missing death is unavailable, not zero, even after final closure") {
    val result = analyze(fixture("snake-negative", "request"), lifetimeProfile)
    assertEquals(metric(result, "lifetime.player", "p2").reason, Some("missing_end"))
  }
  test("death before start, after finish or repeated is invalid") {
    val base = fixture("snake-positive", "request")
    val before = editEvent(base, 6)(e => update(e, "timestamp", Json.Str("2026-10-17T17:59:59Z")))
    val after = editEvent(base, 6)(e => update(e, "timestamp", Json.Str("2026-10-17T18:01:00Z")))
    assertEquals(metric(analyze(before, lifetimeProfile), "lifetime.player").reason, Some("invalid_time"))
    assertEquals(metric(analyze(after, lifetimeProfile), "lifetime.player").reason, Some("invalid_time"))
    val repeated = editEvent(base, 7)(e => update(e, "playerId", Json.Str("p1")))
    assertEquals(metric(analyze(repeated, lifetimeProfile), "lifetime.player").reason, Some("repeated_end"))
  }
  test("an observed death is provisional while the match remains open") {
    val base = fixture("snake-positive", "request")
    val request = update(update(base, "state", Json.Str("open")), "events",
      Json.Arr(array(field(base, "events")).dropRight(1)))
    assertEquals(metric(analyze(request, lifetimeProfile), "lifetime.player").status, "provisional")
  }
  test("duration uses instants, including equal timestamps and equivalent time zones") {
    val request = editEvent(fixture("race-final", "request"), 4)(e =>
      update(e, "timestamp", Json.Str("2026-10-17T12:00:00-06:00")))
    assertEquals(metric(analyze(request), "duration.match", "race-live").value, Some(BigDecimal(0)))
  }
  test("fractional duration rounds HALF_EVEN only after subtraction") {
    val request = editEvent(fixture("race-final", "request"), 4)(e =>
      update(e, "timestamp", Json.Str("2026-10-17T18:00:01.1234565Z")))
    assertEquals(metric(analyze(request), "duration.match", "race-live").value, Some(BigDecimal("1.123456")))
  }
