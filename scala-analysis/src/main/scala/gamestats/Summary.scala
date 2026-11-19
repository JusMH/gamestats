package gamestats

/** Cross-match analytics remain pure and belong to Scala, never the console. */
object Summary:
  private def counts(matches: Vector[Snapshot]): Map[String, Json] = Map(
    "started" -> Json.IntNum(matches.size),
    "active" -> Json.IntNum(matches.count(_.state == "open")),
    "finished" -> Json.IntNum(matches.count(_.state == "closed")),
    "events" -> Json.IntNum(matches.map(_.events.size.toLong).sum))

  def analyze(input: Json, engine: Engine): Checked[Json] =
    for
      fields <- Json.obj(input)
      raw <- Json.read(fields, "matches")(Json.arr)
      snapshots <- Json.traverse(raw)(Decoding.snapshot)
      _ <- Json.ensure(snapshots.map(s => (s.gameId, s.matchId)).distinct.size == snapshots.size,
        "duplicate_match", "matches")
      results <- Json.traverse(snapshots)(engine.analyze)
    yield
      val games = (engine.profiles.keySet ++ snapshots.map(_.gameId)).toVector.sorted.map { gameId =>
        val games = snapshots.filter(_.gameId == gameId)
        val analyzed = results.filter(_.gameId == gameId)
        val definitions = engine.profiles.get(gameId).toVector.flatMap(_.capabilities).flatMap {
          case cap: EventMetrics => cap.metrics.filter(d => d.scope == Scope.Player && d.operation != Operation.Avg)
          case _ => Vector.empty
        }
        val aggregate = definitions.map { definition =>
          val rows = analyzed.flatMap(_.metrics).filter(_.id == definition.id)
          val available = rows.filter(m => m.status == "final" || m.status == "provisional")
          Json.Obj(Map("id" -> Json.Str(definition.id),
            "label" -> Json.Str(definition.label.replace("por jugador", "totales")),
            "value" -> Json.num(available.flatMap(_.value).foldLeft(BigDecimal(0))(_ + _)),
            "unit" -> Json.Str(definition.unit),
            "invalidSamples" -> Json.IntNum(rows.count(m => m.status == "invalid" || m.status == "unavailable"))))
        }
        val durations = analyzed.flatMap(_.metrics).filter(m => m.id == "duration.match" && m.status == "final")
        val duration = Json.Obj(Map("id" -> Json.Str("duration.total"), "label" -> Json.Str("Duración finalizada"),
          "value" -> Json.num(durations.flatMap(_.value).foldLeft(BigDecimal(0))(_ + _)),
          "unit" -> Json.Str("seconds"), "invalidSamples" -> Json.IntNum(0)))
        Json.Obj(counts(games) ++ Map("gameId" -> Json.Str(gameId), "metrics" -> Json.Arr(aggregate :+ duration)))
      }
      Json.Obj(Map("source" -> Json.Str("GameStats / Scala"),
        "scope" -> Json.Str("current_process"), "consistency" -> Json.Str("per_match_snapshot"),
        "totals" -> Json.Obj(counts(snapshots)), "games" -> Json.Arr(games),
        "matches" -> Json.Arr(snapshots.sortBy(s => (s.gameId, s.matchId)).map(s => Json.Obj(Map(
          "gameId" -> Json.Str(s.gameId), "matchId" -> Json.Str(s.matchId),
          "state" -> Json.Str(s.state), "eventCount" -> Json.IntNum(s.events.size)))))))
