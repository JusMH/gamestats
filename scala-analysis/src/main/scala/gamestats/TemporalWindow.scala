package gamestats

import java.time.Duration
import scala.annotation.tailrec

/** Stateless cardinality window, greedy consumption with an inclusive bound. */
object TemporalWindow:
  def analyze(cap: Window, ctx: AnalysisContext): Vector[Pattern] =
    val selected = ctx.snapshot.events.filter(_.kind == cap.eventType)
    val grouped = selected.groupBy(_.playerId)
    val playerError = ctx.playerError(selected)
    ctx.players.map { player =>
      val candidates = ctx.ordered(grouped.getOrElse(Some(player), Vector.empty))
      val initial = Pattern(cap.id, cap.label, Scope.Player, player, ctx.status,
        Some(0), Vector.empty, ctx.source(cap), ctx.basis)
      playerError.orElse(ctx.timeError(candidates)) match
        case Some(reason) => initial.copy(status = "invalid", count = None, reason = Some(reason))
        case None if cap.availability == Availability.Final && ctx.snapshot.state == "open" =>
          initial.copy(status = "unavailable", count = None, reason = Some("awaiting_finish"))
        case None =>
          val occurrences = consume(candidates, cap.size, cap.seconds)
          initial.copy(count = Some(occurrences.size), occurrences = occurrences)
    }

  private def consume(events: Vector[Event], size: Int, seconds: BigDecimal): Vector[Occurrence] =
    @tailrec
    def loop(offset: Int, found: Vector[Occurrence]): Vector[Occurrence] =
      if offset + size > events.size then found
      else
        val first = events(offset)
        val last = events(offset + size - 1)
        val elapsed = Duration.between(first.timestamp, last.timestamp)
        val exactSeconds = java.math.BigDecimal.valueOf(elapsed.getSeconds)
          .add(java.math.BigDecimal.valueOf(elapsed.getNano.toLong, 9))
        if exactSeconds.compareTo(seconds.bigDecimal) <= 0 then
          val group = events.slice(offset, offset + size)
          val occurrence = Occurrence(group.map(_.eventId), first.wireTimestamp, last.wireTimestamp)
          loop(offset + size, found :+ occurrence)
        else loop(offset + 1, found)
    loop(0, Vector.empty)
