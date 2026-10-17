package gamestats

import java.time.Instant

class PatternsSuite extends munit.FunSuite:
  import TestData.*
  private val loaded = right(Profiles.load(profiles))
  private val service = new Engine(loaded)
  private val origin = snapshot("snake-positive").events.head.timestamp
  private def at(seconds: String): Instant =
    origin.plusNanos((BigDecimal(seconds) * BigDecimal(1000000000L)).toLongExact)
  private def timed(event: Event, time: Instant): Event =
    event.copy(timestamp = time, timestampText = time.toString)
  private def run(s: Snapshot): AnalysisResult = right(service.analyze(s))
  private def pattern(r: AnalysisResult, id: String = "apples.burst", player: String = "p1"): Pattern =
    r.patterns.find(p => p.id == id && p.subjectId == player).get
  private def ids(p: Pattern): Vector[Vector[String]] = p.occurrences.map(_.eventIds)
  private def snake(times: Vector[String], state: String = "open", finish: String = "100"): Snapshot =
    val base = snapshot("snake-positive")
    val apple = base.events(1)
    val events = times.zipWithIndex.map { (time, i) =>
      timed(apple.copy(eventId = "e" + i, data = Json.Obj(Map("points" -> Json.IntNum(1)))), at(time))
    }
    val closed = if state == "closed" then Vector(timed(base.events.last, at(finish))) else Vector.empty
    base.copy(state = state, events = Vector(base.events.head) ++ events ++ closed)

  test("inclusive exact boundary and one nanosecond outside are distinguished") {
    assertEquals(pattern(run(snake(Vector("1", "6", "11")))).count, Some(1))
    assertEquals(pattern(run(snake(Vector("1", "6", "11.000000001")))).count, Some(0))
  }
  test("greedy consumption never reuses events in overlapping candidates") {
    val result = pattern(run(snake(Vector("1", "2", "3", "4", "5", "6", "7"))))
    assertEquals(ids(result), Vector(Vector("e0", "e1", "e2"), Vector("e3", "e4", "e5")))
    assertEquals(result.occurrences.flatMap(_.eventIds).distinct.size, 6)
  }
  test("failed early candidate advances by one, not by the window size") {
    assertEquals(ids(pattern(run(snake(Vector("1", "20", "21", "22"))))),
      Vector(Vector("e1", "e2", "e3")))
  }
  test("9000 candidates are consumed without recursion overflow or duplicated detections") {
    val result = pattern(run(snake(Vector.fill(9000)("1"))))
    assertEquals(result.count, Some(3000))
    assertEquals(result.occurrences.flatMap(_.eventIds).distinct.size, 9000)
  }
  test("N and the window duration come from configuration, not from Snake constants") {
    val profile = loaded("snake")
    val configured = profile.copy(capabilities = profile.capabilities.map {
      case cap: Window => cap.copy(size = 2, seconds = BigDecimal("0.000000001"))
      case cap => cap
    })
    val engine = new Engine(Map("snake" -> configured))
    val result = right(engine.analyze(snake(Vector("1", "1.000000001", "2"))))
    assertEquals(ids(pattern(result)), Vector(Vector("e0", "e1")))
  }
  test("timestamps tied in different zones retain original acceptance order and real times") {
    val base = snake(Vector("2", "2", "2"))
    val texts = Vector("2026-10-17T18:00:02Z", "2026-10-17T12:00:02-06:00",
      "2026-10-17T19:00:02+01:00")
    val names = Vector("z", "a", "m")
    val events = base.events.tail.zipWithIndex.map { (e, i) =>
      e.copy(eventId = names(i), timestampText = texts(i))
    }
    val result = pattern(run(base.copy(events = base.events.take(1) ++ events)))
    assertEquals(ids(result), Vector(names))
    assertEquals(result.occurrences.head.startTimestamp, texts.head)
    assertEquals(result.occurrences.head.endTimestamp, texts.last)
  }
  test("a late event in an open snapshot can replace provisional occurrences") {
    val before = snake(Vector("5", "11"))
    val initial = run(before)
    assertEquals(pattern(initial).count, Some(0))
    val after = before.copy(events = before.events :+
      timed(before.events(1).copy(eventId = "late-1"), at("1")))
    val first = run(after)
    assertEquals(ids(pattern(first)), Vector(Vector("late-1", "e0", "e1")))
    val next = after.copy(events = after.events :+
      timed(before.events(1).copy(eventId = "late-2"), at("2")))
    assertEquals(ids(pattern(run(next))), Vector(Vector("late-1", "late-2", "e0")))
    assertEquals(pattern(run(next)).count, Some(1))
    assertEquals(run(after).json, first.json)
    assertEquals(run(before).json, initial.json)
  }
  test("numeric errors do not corrupt independent temporal evidence") {
    val base = snake(Vector("1", "2", "3"))
    val bad = base.copy(events = base.events.updated(1,
      base.events(1).copy(data = Json.Obj(Map("points" -> Json.Str("bad"))))))
    val result = run(bad)
    assertEquals(metric(result, "points.player").status, "invalid")
    assertEquals(ids(pattern(result)), Vector(Vector("e0", "e1", "e2")))
    assertEquals(pattern(result).status, "provisional")
  }
  test("different players never share candidates") {
    val base = snake(Vector("1", "2", "3", "4", "5"))
    val mixed = base.copy(events = base.events.zipWithIndex.map { (e, i) =>
      if i == 2 || i == 4 then e.copy(playerId = Some("p2")) else e
    })
    assertEquals(ids(pattern(run(mixed))), Vector(Vector("e0", "e2", "e4")))
    assertEquals(pattern(run(mixed), player = "p2").count, Some(0))
  }
  test("fewer than N events is a valid zero, not an invented occurrence") {
    val result = pattern(run(snake(Vector("1", "2"))))
    assertEquals(result.count, Some(0))
    assertEquals(result.status, "provisional")
    assertEquals(result.reason, None)
  }
  test("invalid or missing player invalidates dependent patterns explicitly") {
    Vector(None, Some("outsider")).foreach { player =>
      val base = snake(Vector("1", "2", "3"))
      val result = run(base.copy(events = base.events.updated(1, base.events(1).copy(playerId = player))))
      val row = pattern(result)
      assertEquals(row.status, "invalid")
      assertEquals(row.count, None)
      assertEquals(row.occurrences, Vector.empty)
      assertEquals(row.reason, Some(if player.isEmpty then "missing_player" else "unknown_player"))
    }
  }
  test("selected events outside temporal lifecycle bounds are invalid") {
    val before = pattern(run(snake(Vector("-0.000000001", "1", "2"))))
    val after = pattern(run(snake(Vector("1", "2", "11"), "closed", "10")))
    Vector(before, after).foreach { row =>
      assertEquals(row.status, "invalid")
      assertEquals(row.reason, Some("invalid_time"))
      assertEquals(row.count, None)
    }
  }
  test("open detections are provisional and closed detections are final") {
    assertEquals(pattern(run(snake(Vector("1", "2", "3")))).status, "provisional")
    assertEquals(pattern(run(snake(Vector("1", "2", "3"), "closed", "4"))).status, "final")
  }
  test("future final-only windows expose unavailable while open, without fake zeros") {
    val s = snake(Vector("1", "2", "3"))
    val profile = loaded("snake")
    val context = AnalysisContext(s, profile, Vector("p1", "p2"))
    val cap = profile.capabilities.collectFirst { case w: Window => w }.get
    // Direct executor check only. The current approved DSL permits live windows only.
    val result = TemporalWindow.analyze(cap.copy(availability = Availability.Final), context).head
    assertEquals(result.status, "unavailable")
    assertEquals(result.count, None)
    assertEquals(result.reason, Some("awaiting_finish"))
  }

  private def board(moves: Vector[(String, Int, Int)], state: String = "closed"): Snapshot =
    val base = snapshot("ttt-win")
    val template = base.events(1)
    val events = moves.zipWithIndex.map { case ((player, row, column), i) =>
      timed(template.copy(eventId = "m" + (i + 1), playerId = Some(player),
        data = Json.Obj(Map("turn" -> Json.IntNum(i + 1), "row" -> Json.IntNum(row),
          "column" -> Json.IntNum(column), "symbol" -> Json.Str(if player == "alex" then "X" else "O")))),
        origin.plusSeconds(i + 1L))
    }
    val end = if state == "closed" then Vector(timed(base.events.last,
      origin.plusSeconds(events.size + 1L))) else Vector.empty
    base.copy(state = state, events = Vector(base.events.head) ++ events ++ end)
  private def boardMetric(result: AnalysisResult): Metric = metric(result, "board.result", result.matchId)
  private def boardPattern(result: AnalysisResult): Pattern =
    pattern(result, "board.winning_line", result.matchId)
  private def cells(result: AnalysisResult): Json =
    field(boardPattern(result).occurrences.head.details.get, "cells")
  private def invalidBoard(s: Snapshot, reason: String = "invalid_board"): Unit =
    val result = run(s)
    assertEquals(boardMetric(result).status, "invalid")
    assertEquals(boardMetric(result).reason, Some(reason))
    assertEquals(boardPattern(result).count, None)
    assertEquals(boardPattern(result).reason, Some(reason))
    assertEquals(boardPattern(result).occurrences, Vector.empty)

  val winningLines = Vector(
    Vector((0,0),(0,1),(0,2)), Vector((1,0),(1,1),(1,2)), Vector((2,0),(2,1),(2,2)),
    Vector((0,0),(1,0),(2,0)), Vector((0,1),(1,1),(2,1)), Vector((0,2),(1,2),(2,2)),
    Vector((0,0),(1,1),(2,2)), Vector((0,2),(1,1),(2,0)))
  winningLines.zipWithIndex.foreach { (line, i) =>
    test("winning row/column/diagonal " + i + " has exact evidence") {
      val result = run(board(line.map((r,c) => ("alex",r,c))))
      assertEquals(boardMetric(result).structuredValue,
        Some(parse("""{"outcome":"win","winnerId":"alex"}""").asInstanceOf[Json.Obj]))
      assertEquals(cells(result), Json.Arr(line.map((r,c) =>
        Json.Obj(Map("row" -> Json.IntNum(r), "column" -> Json.IntNum(c))))))
      assertEquals(ids(boardPattern(result)), Vector(Vector("m1","m2","m3")))
    }
  }
  test("one move completing a row and a column chooses the documented row first") {
    val moves = Vector((0,0),(0,1),(1,2),(2,2),(0,2)).map((r,c) => ("alex",r,c))
    val result = run(board(moves))
    assertEquals(cells(result), parse("""[{"row":0,"column":0},{"row":0,"column":1},{"row":0,"column":2}]"""))
    assertEquals(ids(boardPattern(result)), Vector(Vector("m1","m2","m5")))
  }
  test("later same-player lines do not replace or duplicate the first line") {
    val moves = Vector((0,0),(0,1),(0,2),(1,0),(2,0)).map((r,c) => ("alex",r,c))
    assertEquals(ids(boardPattern(run(board(moves)))), Vector(Vector("m1","m2","m3")))
  }
  test("opposing winners are ambiguous even when one line appeared first") {
    val moves = Vector(("alex",0,0),("alex",0,1),("alex",0,2),
      ("bea",1,0),("bea",1,1),("bea",1,2))
    invalidBoard(board(moves), "ambiguous_winner")
  }
  test("occupied cells are invalid, including moves after an earlier win") {
    invalidBoard(board(Vector(("alex",0,0),("bea",0,0))))
    invalidBoard(board(Vector(("alex",0,0),("alex",0,1),("alex",0,2),("bea",0,0))))
  }
  test("out-of-order acceptance is reconstructed by UTC time") {
    val base = snapshot("ttt-win")
    val reordered = base.copy(events = Vector(base.events.head, base.events(3), base.events(1),
      base.events(2), base.events(5), base.events(4), base.events.last))
    assertEquals(run(reordered).json, fixture("ttt-win", "expected"))
  }
  test("simultaneous moves use acceptance order, without inventing turn causality") {
    val base = board(Vector(("alex",0,0),("alex",0,1),("alex",0,2)))
    val tied = base.copy(events = base.events.map { e =>
      if e.kind == "MOVE_PLACED" then timed(e, at("1")) else e
    })
    assertEquals(ids(boardPattern(run(tied))), Vector(Vector("m1","m2","m3")))
    invalidBoard(tied.copy(events = tied.events.updated(1, tied.events(2)).updated(2, tied.events(1))))
  }
  val badMoveData = Vector(
    "row" -> Json.IntNum(3), "column" -> Json.IntNum(-1),
    "row" -> Json.DecNum(BigDecimal("0.5")), "turn" -> Json.IntNum(0),
    "turn" -> Json.IntNum(2), "symbol" -> Json.Str("O"), "symbol" -> Json.Str("Q"))
  badMoveData.zipWithIndex.foreach { case ((key, value), i) =>
    test("invalid move data is explicit " + i) {
      val base = board(Vector(("alex",0,0)))
      val event = base.events(1)
      val changed = event.copy(data = update(event.data, key, value))
      val invalid = base.copy(events = base.events.updated(1, changed))
      invalidBoard(invalid)
      assertEquals(metric(run(invalid), "moves.match", invalid.matchId).value, Some(BigDecimal(1)))
    }
  }
  test("missing fields, absent player and symbol assignment mismatches are invalid") {
    val base = board(Vector(("alex",0,0)))
    invalidBoard(base.copy(events = base.events.updated(1, base.events(1).copy(data = Json.Obj(Map.empty)))))
    invalidBoard(base.copy(events = base.events.updated(1, base.events(1).copy(playerId = None))))
    invalidBoard(base.copy(events = base.events.updated(1, base.events(1).copy(playerId = Some("outsider")))))
    val badStart = base.events.head.copy(data = update(base.events.head.data, "symbols",
      parse("""{"alex":"X","bea":"X"}""")))
    invalidBoard(base.copy(events = base.events.updated(0, badStart)))
  }
  test("a closed incomplete board is unavailable, not an invented draw") {
    val base = snapshot("ttt-open")
    val end = timed(snapshot("ttt-win").events.last.copy(matchId = base.matchId), at("6"))
    val result = run(base.copy(state = "closed", events = base.events :+ end))
    assertEquals(boardMetric(result).status, "unavailable")
    assertEquals(boardMetric(result).reason, Some("insufficient_evidence"))
    assertEquals(boardMetric(result).structuredValue, None)
    assertEquals(boardPattern(result).count, Some(0))
    assertEquals(boardPattern(result).status, "final")
  }
  test("a win is visible provisionally before MATCH_FINISHED") {
    val base = snapshot("ttt-win")
    val result = run(base.copy(state = "open", events = base.events.dropRight(1)))
    assertEquals(boardMetric(result).status, "provisional")
    assertEquals(boardPattern(result).status, "provisional")
    assertEquals(boardPattern(result).count, Some(1))
  }
  test("the analyzer does not silently enforce X-first or turn alternation") {
    val result = run(board(Vector(("bea",0,0),("bea",0,1),("bea",0,2))))
    assertEquals(boardMetric(result).status, "final")
    assertEquals(field(boardMetric(result).structuredValue.get, "winnerId"), Json.Str("bea"))
  }
  test("another game can select the same specialized analyzer by analyzerId") {
    val profile = loaded("tic-tac-toe").copy(gameId = "another-board-game")
    val base = snapshot("ttt-win")
    val renamed = base.copy(gameId = profile.gameId,
      events = base.events.map(_.copy(gameId = profile.gameId)))
    val result = right(new Engine(Map(profile.gameId -> profile)).analyze(renamed))
    assertEquals(boardPattern(result).count, Some(1))
    assertEquals(boardMetric(result).source.kind, SourceKind.Analyzer)
  }
  test("concurrent pure analyses of all games are deterministic and isolated") {
    import scala.concurrent.{Await, ExecutionContext, Future}
    import scala.concurrent.duration.*
    given ExecutionContext = ExecutionContext.global
    val ids = supported ++ supported
    val analyses = ids.map(id => Future(right(service.analyze(snapshot(id))).json))
    val results = Await.result(Future.sequence(analyses), 15.seconds)
    results.zip(ids).foreach((actual,id) => assertEquals(actual, fixture(id,"expected")))
  }

  test("invalid temporal parameters and unsupported versions never execute silently") {
    val configured = file(profiles.resolve("snake.json"))
    val caps = array(field(configured, "capabilities"))
    val window = caps.last
    val changes = Vector(
      "size" -> Json.IntNum(1), "windowSeconds" -> Json.IntNum(-1),
      "overlap" -> Json.Str("reuse"), "scope" -> Json.Str("match"))
    changes.foreach { (key, value) =>
      val changed = update(window, "parameters", update(field(window, "parameters"), key, value))
      val invalid = update(configured, "capabilities", Json.Arr(caps.updated(caps.size - 1, changed)))
      assert(Decoding.profile(invalid).isLeft)
    }
    Vector(update(window, "templateId", Json.Str("temporal-window/v2")),
      update(window, "availability", Json.Str("final"))).foreach { changed =>
      val invalid = update(configured, "capabilities", Json.Arr(caps.updated(caps.size - 1, changed)))
      assert(Decoding.profile(invalid).isLeft)
    }
  }

  test("the board validates its declared start roster even without profile playersField") {
    val profile = loaded("tic-tac-toe").copy(playersField = None)
    val base = snapshot("ttt-win")
    val missing = base.copy(events = base.events.updated(0,
      base.events.head.copy(data = remove(base.events.head.data, "players"))))
    val result = right(new Engine(Map(profile.gameId -> profile)).analyze(missing))
    assertEquals(boardMetric(result).reason, Some("invalid_board"))
  }
