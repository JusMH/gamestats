package gamestats

import java.math.RoundingMode
import java.time.Duration

case class AnalysisContext(snapshot: Snapshot, profile: Profile, players: Vector[String]):
  val basis = Basis(snapshot.events.size, snapshot.events.last.eventId)
  def status: String = if snapshot.state == "closed" then "final" else "provisional"
  def subjects(scope: Scope): Vector[String] =
    if scope == Scope.Match then Vector(snapshot.matchId) else players
  def source(cap: Capability): Source =
    Source(cap.templateId, cap.instanceId, profile.version)
  def playerError(events: Vector[Event]): Option[String] =
    if events.exists(_.playerId.forall(_.isEmpty)) then Some("missing_player")
    else if profile.playersField.isDefined && events.exists(e => !players.contains(e.playerId.get))
      then Some("unknown_player")
    else None

/** Registered pure interpreters. Configuration dispatch never depends on gameId. */
object Capabilities:
  type Runner = (Capability, AnalysisContext) => Vector[Metric]
  val registry: Map[String, Runner] = Map(
    "event-metrics/v1" -> ((cap, ctx) => cap match
      case metrics: EventMetrics => eventMetrics(metrics, ctx)
      case _ => throw new IllegalStateException("Capability registry type mismatch")),
    "duration/v1" -> ((cap, ctx) => cap match
      case interval: Interval => duration(interval, ctx)
      case _ => throw new IllegalStateException("Capability registry type mismatch")))

  private def unavailable(metric: Metric, reason: String): Metric =
    metric.copy(value = None, status = "unavailable", reason = Some(reason))
  private def invalid(metric: Metric, reason: String): Metric =
    metric.copy(value = None, status = "invalid", reason = Some(reason))
  private def availability(metric: Metric, cap: Capability, ctx: AnalysisContext): Metric =
    if metric.status != "invalid" && cap.availability == Availability.Final &&
      ctx.snapshot.state == "open" then unavailable(metric, "awaiting_finish")
    else metric

  private def eventMetrics(cap: EventMetrics, ctx: AnalysisContext): Vector[Metric] =
    cap.metrics.flatMap { definition =>
      val selected = ctx.snapshot.events.filter(_.kind == definition.eventType)
      val byPlayer = selected.groupBy(_.playerId)
      val scopeError = if definition.scope == Scope.Player then ctx.playerError(selected) else None
      ctx.subjects(definition.scope).map { subject =>
        val samples = if definition.scope == Scope.Match then selected
          else byPlayer.getOrElse(Some(subject), Vector.empty)
        val initial = Metric(definition.id, definition.label, definition.scope, subject,
          None, definition.unit, ctx.status, ctx.source(cap), ctx.basis)
        val calculated = scopeError match
          case Some(reason) => invalid(initial, reason)
          case None if definition.operation == Operation.Count =>
            initial.copy(value = Some(BigDecimal(samples.size)))
          case None =>
            val values = Json.traverse(samples) { event =>
              for
                field <- Json.pointer(event.document, definition.field.get)
                  .toRight(ValidationError("missing_field", definition.field.get))
                value <- Json.number(field)
                _ <- Json.ensure(definition.numberKind.contains(NumberKind.Decimal) || value.isWhole,
                  "invalid_number", definition.field.get)
                _ <- Json.ensure(definition.minimum.forall(value >= _),
                  "invalid_number", definition.field.get)
              yield value
            }
            values match
              case Left(error) => invalid(initial, error.code)
              case Right(values) if definition.operation == Operation.Avg && values.isEmpty =>
                unavailable(initial, "no_samples")
              case Right(values) =>
                // Java BigDecimal addition is exact; no DECIMAL128 intermediate rounding.
                val total = values.foldLeft(java.math.BigDecimal.ZERO)((sum, n) => sum.add(n.bigDecimal))
                val value = if definition.operation == Operation.Avg
                  then total.divide(java.math.BigDecimal.valueOf(values.size.toLong), 6, RoundingMode.HALF_EVEN)
                  else total
                initial.copy(value = Some(BigDecimal(value.toString, java.math.MathContext.UNLIMITED)))
        availability(calculated, cap, ctx)
      }
    }

  private def duration(cap: Interval, ctx: AnalysisContext): Vector[Metric] =
    val start = ctx.snapshot.events.head.timestamp
    val finish = ctx.snapshot.events.find(_.kind == "MATCH_FINISHED").map(_.timestamp)
    val selected = ctx.snapshot.events.filter(_.kind == cap.endEventType)
    val scopeError = if cap.scope == Scope.Player then ctx.playerError(selected) else None
    ctx.subjects(cap.scope).map { subject =>
      val ends = selected.filter(e => cap.scope == Scope.Match || e.playerId.contains(subject))
      val initial = Metric(cap.id, cap.label, cap.scope, subject, None, "seconds",
        ctx.status, ctx.source(cap), ctx.basis)
      val calculated = scopeError match
        case Some(reason) => invalid(initial, reason)
        case None if ends.size > 1 => invalid(initial, "repeated_end")
        case None if ends.isEmpty => unavailable(initial,
          if ctx.snapshot.state == "open" then "awaiting_end" else "missing_end")
        case None =>
          val end = ends.head.timestamp
          if end.isBefore(start) || finish.exists(end.isAfter) then invalid(initial, "invalid_time")
          else
            val delta = Duration.between(start, end)
            val seconds = java.math.BigDecimal.valueOf(delta.getSeconds)
              .add(java.math.BigDecimal.valueOf(delta.getNano.toLong, 9))
              .setScale(6, RoundingMode.HALF_EVEN)
            initial.copy(value = Some(BigDecimal(seconds.toString, java.math.MathContext.UNLIMITED)))
      availability(calculated, cap, ctx)
    }

final class Engine(val profiles: Map[String, Profile]):
  def analyze(snapshot: Snapshot): Checked[AnalysisResult] =
    if snapshot.events.isEmpty then Left(ValidationError("event_limit", "events"))
    else profiles.get(snapshot.gameId) match
      case None => Right(AnalysisResult(snapshot.gameId, snapshot.matchId, snapshot.state, Vector.empty))
      case Some(profile) if profile.pending.nonEmpty =>
        Left(ValidationError("capability_not_implemented", profile.pending.distinct.mkString(",")))
      case Some(profile) =>
        for
          players <- participants(snapshot, profile)
          _ <- Json.ensure(profile.capabilities.map(_.outputIds.size).sum.toLong *
            math.max(1, players.size) <= 20000, "result_limit", "metrics")
        yield
          val context = AnalysisContext(snapshot, profile, players)
          val metrics = profile.capabilities.flatMap(cap =>
            Capabilities.registry(cap.templateId)(cap, context))
          AnalysisResult(snapshot.gameId, snapshot.matchId, snapshot.state,
            metrics.sortWith((a, b) => compareKeys(a, b) < 0))

  // Code-point ordering, independent of JVM locale and UTF-16 surrogate ordering.
  private def compareKeys(a: Metric, b: Metric): Int =
    Vector(a.id -> b.id, a.scope.wire -> b.scope.wire, a.subjectId -> b.subjectId)
      .iterator.map { (left, right) =>
        val xs = left.codePoints().toArray
        val ys = right.codePoints().toArray
        xs.zip(ys).iterator.map((x, y) => Integer.compare(x, y))
          .find(_ != 0).getOrElse(Integer.compare(xs.length, ys.length))
      }.find(_ != 0).getOrElse(0)

  private def participants(snapshot: Snapshot, profile: Profile): Checked[Vector[String]] =
    profile.playersField match
      case Some(path) =>
        val roster = Json.pointer(snapshot.events.head.document, path)
        val decoded = roster.toRight(ValidationError("invalid_roster", "players"))
          .flatMap(Json.arr).flatMap(Json.traverse(_)(Json.str))
        decoded.flatMap(players =>
          Json.ensure(players.nonEmpty && players.size <= 1000 &&
            players.distinct.size == players.size, "invalid_roster", "players").map(_ => players))
          .left.map(_ => ValidationError("invalid_roster", "players"))
      case None =>
        val relevant = profile.capabilities.flatMap(_.playerEventTypes).toSet
        val players = snapshot.events.filter(e => relevant(e.kind))
          .flatMap(_.playerId.filter(_.nonEmpty)).distinct
        Json.ensure(players.size <= 1000, "player_limit", "players").map(_ => players)
