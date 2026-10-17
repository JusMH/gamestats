package gamestats

import java.nio.file.{Files, Path}
import java.time.{LocalDateTime, ZoneOffset}
import scala.jdk.CollectionConverters.*
import scala.util.control.NonFatal

object Decoding:
  import Json.*
  private def choice[A](value: Json, choices: Map[String, A]): Checked[A] =
    str(value).flatMap(s => choices.get(s).toRight(ValidationError("unknown_value", "value")))
  private val scope: Json => Checked[Scope] =
    choice(_, Map("player" -> Scope.Player, "match" -> Scope.Match))
  private val availability: Json => Checked[Availability] =
    choice(_, Map("live" -> Availability.Live, "final" -> Availability.Final))
  private def pointer(value: Json): Checked[String] =
    str(value).flatMap(p => ensure(p.startsWith("/data/") &&
      p.drop(6).nonEmpty && !p.contains("//") && !p.contains("*") &&
      !"""~(?![01])""".r.findFirstIn(p).isDefined, "invalid_pointer", "field").map(_ => p))

  private val timestampShape =
    """^[0-9]{4}-[0-9]{2}-[0-9]{2}T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$""".r
  private def timestamp(value: Json): Checked[java.time.Instant] =
    str(value).flatMap { text =>
      try
        if !timestampShape.matches(text) then Left(ValidationError("invalid_timestamp", "timestamp"))
        else
          // Go time.Parse truncates fractions beyond nanoseconds.
          val normalized = text.replaceFirst("""(\.[0-9]{9})[0-9]+""", "$1")
          val split = if normalized.endsWith("Z") then normalized.length - 1 else normalized.length - 6
          val local = LocalDateTime.parse(normalized.substring(0, split)).toInstant(ZoneOffset.UTC)
          val zone = normalized.substring(split)
          val offset = if zone == "Z" then 0 else
            (zone.substring(1, 3).toInt * 3600 + zone.substring(4, 6).toInt * 60) *
              (if zone.head == '-' then -1 else 1)
          Right(local.minusSeconds(offset))
      catch case NonFatal(_) => Left(ValidationError("invalid_timestamp", "timestamp"))
    }

  private def optionalString(value: Json): Checked[String] = value match
    case Str(s) => Right(s)
    case _ => Left(ValidationError("expected_string", "string"))

  def event(value: Json): Checked[Event] =
    for
      fields <- obj(value)
      version <- read(fields, "schemaVersion")(integer)
      _ <- ensure(version == 1, "schema_version", "schemaVersion")
      id <- read(fields, "eventId")(str)
      timeText <- read(fields, "timestamp")(str)
      time <- timestamp(Str(timeText))
      game <- read(fields, "gameId")(str)
      m <- read(fields, "matchId")(str)
      player <- optional(fields, "playerId")(optionalString)
      _ <- optional(fields, "gameVersion")(optionalString)
      kind <- read(fields, "type")(str)
      data <- field(fields, "data")
      _ <- obj(data)
    yield Event(id, time, game, m, player, kind, data, timeText)

  def snapshot(value: Json): Checked[Snapshot] =
    for
      fields <- strict(value, Set("gameId", "matchId", "state", "events"))
      game <- read(fields, "gameId")(str)
      m <- read(fields, "matchId")(str)
      state <- read(fields, "state")(choice(_, Map("open" -> "open", "closed" -> "closed")))
      rawEvents <- read(fields, "events")(arr)
      _ <- ensure(rawEvents.nonEmpty && rawEvents.size <= 10000, "event_limit", "events")
      events <- traverse(rawEvents)(event)
      _ <- ensure(events.forall(e => e.gameId == game && e.matchId == m), "mixed_match", "events")
      _ <- ensure(events.map(_.eventId).distinct.size == events.size, "duplicate_event", "events")
      _ <- ensure(events.head.kind == "MATCH_STARTED" &&
        events.count(_.kind == "MATCH_STARTED") == 1, "invalid_start", "events")
      finishes = events.count(_.kind == "MATCH_FINISHED")
      _ <- ensure(if state == "open" then finishes == 0
        else finishes == 1 && events.last.kind == "MATCH_FINISHED", "invalid_finish", "events")
    yield Snapshot(game, m, state, events)

  private def metric(value: Json): Checked[MetricConfig] =
    for
      fields <- obj(value)
      op <- read(fields, "operation")(choice(_, Map("count" -> Operation.Count,
        "sum" -> Operation.Sum, "avg" -> Operation.Avg)))
      base = Set("id", "label", "eventType", "operation", "scope", "unit")
      _ <- strict(value, if op == Operation.Count then base else base ++ Set("field", "numberKind"),
        if op == Operation.Count then Set.empty else Set("minimum"))
      id <- read(fields, "id")(str)
      label <- read(fields, "label")(str)
      kind <- read(fields, "eventType")(str)
      s <- read(fields, "scope")(scope)
      unit <- read(fields, "unit")(str)
      path <- optional(fields, "field")(pointer)
      numberKind <- optional(fields, "numberKind")(choice(_, Map(
        "integer" -> NumberKind.Integer, "decimal" -> NumberKind.Decimal)))
      minimum <- optional(fields, "minimum")(number)
    yield MetricConfig(id, label, kind, op, s, unit, path, numberKind, minimum)

  private def eventMetrics(value: Json): Checked[Capability] =
    for
      fields <- strict(value, Set("templateId", "instanceId", "availability", "metrics"))
      id <- read(fields, "instanceId")(str)
      available <- read(fields, "availability")(availability)
      raw <- read(fields, "metrics")(arr)
      _ <- ensure(raw.nonEmpty && raw.size <= 256, "metric_limit", "metrics")
      metrics <- traverse(raw)(metric)
    yield EventMetrics(id, available, metrics)

  private def interval(value: Json): Checked[Capability] =
    for
      fields <- strict(value, Set("templateId", "instanceId", "availability", "parameters"))
      instance <- read(fields, "instanceId")(str)
      available <- read(fields, "availability")(availability)
      params <- read(fields, "parameters")(strict(_, Set(
        "id", "label", "scope", "startEventType", "endEventType", "unit")))
      id <- read(params, "id")(str)
      label <- read(params, "label")(str)
      s <- read(params, "scope")(scope)
      start <- read(params, "startEventType")(str)
      end <- read(params, "endEventType")(str)
      unit <- read(params, "unit")(str)
      _ <- ensure(start == "MATCH_STARTED" && unit == "seconds" &&
        (if s == Scope.Match then end == "MATCH_FINISHED"
         else end != "MATCH_STARTED" && end != "MATCH_FINISHED"), "invalid_duration", "parameters")
    yield Interval(instance, available, id, label, s, end)

  /** Recognition/validation only: this capability is not executable in 4A. */
  private def window(value: Json): Checked[Capability] =
    for
      fields <- strict(value, Set("templateId", "instanceId", "availability", "parameters"))
      instance <- read(fields, "instanceId")(str)
      available <- read(fields, "availability")(availability)
      params <- read(fields, "parameters")(strict(_, Set(
        "id", "label", "scope", "eventType", "size", "windowSeconds", "overlap")))
      id <- read(params, "id")(str)
      label <- read(params, "label")(str)
      s <- read(params, "scope")(scope)
      eventType <- read(params, "eventType")(str)
      size <- read(params, "size")(integer)
      seconds <- read(params, "windowSeconds")(number)
      overlap <- read(params, "overlap")(str)
      _ <- ensure(available == Availability.Live && s == Scope.Player &&
        size >= 2 && size <= 10000 && seconds >= 0 && overlap == "consume",
        "invalid_window", "parameters")
    yield Window(instance, available, id, label, eventType, size, seconds)

  // A versioned registry of configuration decoders; no gameId dispatch.
  val capabilityDecoders: Map[String, Json => Checked[Capability]] = Map(
    "event-metrics/v1" -> eventMetrics, "duration/v1" -> interval,
    "temporal-window/v1" -> window)

  private def capability(value: Json): Checked[Capability] =
    for
      fields <- obj(value)
      name <- read(fields, "templateId")(str)
      decoder <- capabilityDecoders.get(name).toRight(ValidationError("unknown_template", "templateId"))
      decoded <- decoder(value)
    yield decoded

  private def board(value: Json): Checked[AnalyzerConfig] =
    for
      fields <- strict(value, Set("analyzerId", "instanceId", "availability", "parameters"))
      id <- read(fields, "instanceId")(str)
      available <- read(fields, "availability")(availability)
      _ <- ensure(available == Availability.Live, "invalid_availability", "availability")
      params <- read(fields, "parameters")(strict(_, Set("resultMetricId", "patternId")))
      result <- read(params, "resultMetricId")(str)
      pattern <- read(params, "patternId")(str)
    yield Board(id, result, pattern)

  val analyzerDecoders: Map[String, Json => Checked[AnalyzerConfig]] =
    Map("tic-tac-toe-line/v1" -> board)

  private def analyzer(value: Json): Checked[AnalyzerConfig] =
    for
      fields <- obj(value)
      id <- read(fields, "analyzerId")(str)
      decode <- analyzerDecoders.get(id).toRight(ValidationError("unknown_analyzer", "analyzerId"))
      result <- decode(value)
    yield result

  def profile(value: Json): Checked[Profile] =
    for
      fields <- strict(value, Set("profileSchemaVersion", "profileVersion", "gameId",
        "capabilities", "specializedAnalyzers"), Set("playersField"))
      version <- read(fields, "profileSchemaVersion")(integer)
      _ <- ensure(version == 1, "profile_schema_version", "profileSchemaVersion")
      revision <- read(fields, "profileVersion")(str)
      _ <- ensure("""[0-9]+\.[0-9]+\.[0-9]+""".r.matches(revision), "profile_version", "profileVersion")
      game <- read(fields, "gameId")(str)
      players <- optional(fields, "playersField")(pointer)
      rawCaps <- read(fields, "capabilities")(arr)
      rawAnalyzers <- read(fields, "specializedAnalyzers")(arr)
      _ <- ensure(rawCaps.size + rawAnalyzers.size <= 128, "capability_limit", "capabilities")
      caps <- traverse(rawCaps)(capability)
      analyzers <- traverse(rawAnalyzers)(analyzer)
      instances = caps.map(_.instanceId) ++ analyzers.map(_.instanceId)
      ids = caps.flatMap(_.outputIds) ++ analyzers.flatMap(_.outputIds)
      _ <- ensure(instances.distinct.size == instances.size, "duplicate_instance", "instanceId")
      _ <- ensure(ids.distinct.size == ids.size && ids.size <= 1000, "duplicate_result", "id")
    yield Profile(game, revision, players, caps, analyzers)

object Profiles:
  def fromJson(values: Vector[Json]): Checked[Map[String, Profile]] =
    for
      profiles <- Json.traverse(values)(Decoding.profile)
      _ <- Json.ensure(profiles.map(_.gameId).distinct.size == profiles.size,
        "duplicate_game", "gameId")
    yield profiles.map(p => p.gameId -> p).toMap

  def load(directory: Path): Checked[Map[String, Profile]] =
    try
      if !Files.isDirectory(directory) then Left(ValidationError("profile_directory", "profiles"))
      else
        val stream = Files.list(directory)
        val files = try stream.iterator().asScala.filter(_.toString.endsWith(".json")).toVector.sortBy(_.toString)
          finally stream.close()
        for
          _ <- Json.ensure(files.nonEmpty && files.size <= 128, "profile_limit", "profiles")
          json <- Json.traverse(files) { file =>
            if Files.size(file) > 1048576 then Left(ValidationError("profile_size", file.getFileName.toString))
            else Json.parse(Files.readAllBytes(file)).left.map(_.under(file.getFileName.toString))
          }
          profiles <- fromJson(json)
        yield profiles
    catch case NonFatal(_) => Left(ValidationError("profile_io", "profiles"))
