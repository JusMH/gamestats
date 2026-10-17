package gamestats

import java.time.Instant

enum Scope:
  case Player, Match
  def wire: String = if this == Player then "player" else "match"
enum Availability:
  case Live, Final
enum Operation:
  case Count, Sum, Avg
enum NumberKind:
  case Integer, Decimal

case class Event(eventId: String, timestamp: Instant, gameId: String,
  matchId: String, playerId: Option[String], kind: String, data: Json):
  def document: Json = Json.Obj(Map("data" -> data))

case class Snapshot(gameId: String, matchId: String, state: String, events: Vector[Event])
case class MetricConfig(id: String, label: String, eventType: String,
  operation: Operation, scope: Scope, unit: String, field: Option[String],
  numberKind: Option[NumberKind], minimum: Option[BigDecimal])

sealed trait Capability:
  def templateId: String
  def instanceId: String
  def availability: Availability
  def outputIds: Vector[String]
  def playerEventTypes: Set[String]
case class EventMetrics(instanceId: String, availability: Availability,
  metrics: Vector[MetricConfig]) extends Capability:
  val templateId = "event-metrics/v1"
  def outputIds = metrics.map(_.id)
  def playerEventTypes = metrics.filter(_.scope == Scope.Player).map(_.eventType).toSet
case class Interval(instanceId: String, availability: Availability, id: String,
  label: String, scope: Scope, endEventType: String) extends Capability:
  val templateId = "duration/v1"
  def outputIds = Vector(id)
  def playerEventTypes = if scope == Scope.Player then Set(endEventType) else Set.empty
case class Window(instanceId: String, availability: Availability, id: String,
  label: String, eventType: String, size: Int, seconds: BigDecimal) extends Capability:
  val templateId = "temporal-window/v1"
  def outputIds = Vector(id)
  def playerEventTypes = Set(eventType)
case class Board(instanceId: String, resultMetricId: String, patternId: String):
  def outputIds = Vector(resultMetricId, patternId)

case class Profile(gameId: String, version: String, playersField: Option[String],
  capabilities: Vector[Capability], analyzers: Vector[Board]):
  def pending: Vector[String] =
    capabilities.collect { case window: Window => window.templateId } ++
      analyzers.map(_ => "tic-tac-toe-line/v1")

case class Source(templateId: String, instanceId: String, profileVersion: String):
  def json: Json = Json.Obj(Map("templateId" -> Json.Str(templateId),
    "instanceId" -> Json.Str(instanceId), "profileVersion" -> Json.Str(profileVersion)))
case class Basis(eventCount: Int, lastEventId: String):
  def json: Json = Json.Obj(Map("eventCount" -> Json.IntNum(eventCount),
    "lastEventId" -> Json.Str(lastEventId)))

case class Metric(id: String, label: String, scope: Scope, subjectId: String,
  value: Option[BigDecimal], unit: String, status: String, source: Source,
  basis: Basis, reason: Option[String] = None):
  def json: Json = Json.Obj(Map("id" -> Json.Str(id), "label" -> Json.Str(label),
    "scope" -> Json.Str(scope.wire), "subjectId" -> Json.Str(subjectId),
    "value" -> value.fold[Json](Json.Null)(Json.num), "unit" -> Json.Str(unit),
    "status" -> Json.Str(status), "source" -> source.json, "basis" -> basis.json) ++
    reason.map(r => "reason" -> Json.Str(r)))

/** No pattern executor is registered in 4A. The type reserves its wire shape. */
case class Occurrence(eventIds: Vector[String], startTimestamp: String, endTimestamp: String):
  def json: Json = Json.Obj(Map("eventIds" -> Json.Arr(eventIds.map(Json.Str(_))),
    "startTimestamp" -> Json.Str(startTimestamp), "endTimestamp" -> Json.Str(endTimestamp)))
case class Pattern(id: String, label: String, scope: Scope, subjectId: String,
  status: String, count: Option[Int], occurrences: Vector[Occurrence],
  source: Source, basis: Basis, reason: Option[String] = None):
  def json: Json = Json.Obj(Map("id" -> Json.Str(id), "label" -> Json.Str(label),
    "scope" -> Json.Str(scope.wire), "subjectId" -> Json.Str(subjectId),
    "status" -> Json.Str(status), "count" -> count.fold[Json](Json.Null)(n => Json.IntNum(n)),
    "occurrences" -> Json.Arr(occurrences.map(_.json)), "source" -> source.json,
    "basis" -> basis.json) ++ reason.map(r => "reason" -> Json.Str(r)))

case class AnalysisResult(gameId: String, matchId: String, state: String,
  metrics: Vector[Metric], patterns: Vector[Pattern] = Vector.empty):
  def json: Json =
    Json.Obj(Map("gameId" -> Json.Str(gameId), "matchId" -> Json.Str(matchId),
      "state" -> Json.Str(state), "metrics" -> Json.Arr(metrics.map(_.json)),
      "patterns" -> Json.Arr(patterns.map(_.json))))
