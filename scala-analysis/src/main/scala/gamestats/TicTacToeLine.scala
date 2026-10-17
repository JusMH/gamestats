package gamestats

/** Specialized analyzers use their own registry; the engine never switches on gameId. */
object Analyzers:
  type Runner = (AnalyzerConfig, AnalysisContext) => Outputs
  val registry: Map[String, Runner] = Map(
    "tic-tac-toe-line/v1" -> ((config, ctx) => config match
      case board: Board => TicTacToeLine.analyze(board, ctx)
      case null => throw new IllegalStateException("Analyzer registry type mismatch")))

/** Checks evidence of lines, not full legal play (alternation or stopping at a win). */
object TicTacToeLine:
  private case class Move(event: Event, position: Int, symbol: String)
  private case class Victory(player: String, line: Vector[Int], events: Vector[Event])
  private case class BoardState(cells: Vector[Option[Move]], first: Option[Victory])

  // Contract tie-break: rows, columns, main diagonal, secondary diagonal.
  private val lines: Vector[Vector[Int]] =
    (0 until 3).map(r => (0 until 3).map(c => r * 3 + c).toVector).toVector ++
      (0 until 3).map(c => (0 until 3).map(r => r * 3 + c).toVector).toVector ++
      Vector(Vector(0, 4, 8), Vector(2, 4, 6))

  private def whole(value: Json): Checked[Int] =
    Json.number(value).flatMap(n => Json.ensure(n.isWhole && n.isValidInt,
      "invalid_board", "coordinate").map(_ => n.toInt))

  private def symbols(ctx: AnalysisContext): Checked[Map[String, String]] =
    for
      rawPlayers <- Json.pointer(ctx.snapshot.events.head.document, "/data/players")
        .toRight(ValidationError("invalid_board", "players"))
      roster <- Json.arr(rawPlayers).flatMap(Json.traverse(_)(Json.str))
      _ <- Json.ensure(roster.size == 2 && roster.distinct.size == 2, "invalid_board", "players")
      raw <- Json.pointer(ctx.snapshot.events.head.document, "/data/symbols")
        .toRight(ValidationError("invalid_board", "symbols"))
      fields <- Json.obj(raw)
      _ <- Json.ensure(fields.keySet == roster.toSet, "invalid_board", "symbols")
      pairs <- Json.traverse(roster) { player =>
        Json.read(fields, player)(Json.str).map(player -> _)
      }
      _ <- Json.ensure(pairs.map(_._2).toSet == Set("X", "O"), "invalid_board", "symbols")
    yield pairs.toMap

  private def move(event: Event, turn: Int, assigned: Map[String, String]): Checked[Move] =
    for
      fields <- Json.obj(event.data)
      actualTurn <- Json.read(fields, "turn")(whole)
      row <- Json.read(fields, "row")(whole)
      column <- Json.read(fields, "column")(whole)
      symbol <- Json.read(fields, "symbol")(Json.str)
      _ <- Json.ensure(actualTurn == turn && row >= 0 && row < 3 &&
        column >= 0 && column < 3 &&
        event.playerId.flatMap(assigned.get).contains(symbol), "invalid_board", "move")
    yield Move(event, row * 3 + column, symbol)

  private def reconstruct(ctx: AnalysisContext): Checked[BoardState] =
    symbols(ctx).flatMap { assigned =>
      val moves = ctx.ordered(ctx.snapshot.events.filter(_.kind == "MOVE_PLACED"))
      moves.zipWithIndex.foldLeft[Checked[BoardState]](
        Right(BoardState(Vector.fill(9)(None), None))) { case (previous, (event, index)) =>
        for
          state <- previous
          next <- move(event, index + 1, assigned).left.map(_ => ValidationError("invalid_board", "move"))
          _ <- Json.ensure(state.cells(next.position).isEmpty, "invalid_board", "occupied_cell")
          cells = state.cells.updated(next.position, Some(next))
          winning = lines.filter(line =>
            line.forall(cells(_).isDefined) && line.map(p => cells(p).get.symbol).distinct.size == 1)
          owners = winning.map(line => cells(line.head).get.event.playerId.get).distinct
          _ <- Json.ensure(owners.size <= 1, "ambiguous_winner", "board")
          first = state.first.orElse(winning.headOption.map { line =>
            Victory(cells(line.head).get.event.playerId.get, line,
              ctx.ordered(line.map(p => cells(p).get.event)))
          })
        yield BoardState(cells, first)
      }
    }.left.map(error => if error.code == "ambiguous_winner" then error
      else ValidationError("invalid_board", error.path))

  def analyze(config: Board, ctx: AnalysisContext): Outputs =
    val source = ctx.source(config)
    val initial = Metric(config.resultMetricId, "Resultado derivado del tablero",
      Scope.Match, ctx.snapshot.matchId, None, "result", ctx.status, source, ctx.basis)
    val pattern = Pattern(config.patternId, "Primera línea ganadora", Scope.Match,
      ctx.snapshot.matchId, ctx.status, Some(0), Vector.empty, source, ctx.basis)
    reconstruct(ctx) match
      case Left(error) =>
        Outputs(Vector(initial.copy(status = "invalid", reason = Some(error.code))),
          Vector(pattern.copy(status = "invalid", count = None, reason = Some(error.code))))
      case Right(state) =>
        val outcome = if state.first.isDefined then "win"
          else if state.cells.forall(_.isDefined) then "draw" else "in_progress"
        val value: Json.Obj = Json.Obj(Map("outcome" -> Json.Str(outcome),
          "winnerId" -> state.first.fold[Json](Json.Null)(v => Json.Str(v.player))))
        val metric = if outcome == "in_progress" && ctx.snapshot.state == "closed"
          then initial.copy(status = "unavailable", reason = Some("insufficient_evidence"))
          else initial.copy(structuredValue = Some(value))
        val occurrences = state.first.toVector.map { victory =>
          val details: Json.Obj = Json.Obj(Map("winnerId" -> Json.Str(victory.player),
            "cells" -> Json.Arr(victory.line.map(position => Json.Obj(Map(
              "row" -> Json.IntNum(position / 3), "column" -> Json.IntNum(position % 3)))))))
          Occurrence(victory.events.map(_.eventId), victory.events.head.wireTimestamp,
            victory.events.last.wireTimestamp, Some(details))
        }
        Outputs(Vector(metric), Vector(pattern.copy(count = Some(occurrences.size), occurrences = occurrences)))
