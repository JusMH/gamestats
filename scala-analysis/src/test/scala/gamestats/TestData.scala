package gamestats

import java.nio.file.{Files, Path}

object TestData:
  val repo: Path = Path.of("..").toAbsolutePath.normalize()
  val fixtures: Path = repo.resolve("docs/fixtures/analysis-v1")
  val profiles: Path = repo.resolve("docs/examples/profiles")
  val supported: Vector[String] = array(field(file(fixtures.resolve("manifest.json")), "cases"))
    .map(value => right(Json.str(field(value, "id"))))
  def right[A](result: Checked[A]): A = result.fold(
    e => throw new AssertionError(e.toString), identity)
  def parse(text: String): Json = right(Json.parse(text))
  def file(path: Path): Json = parse(Files.readString(path))
  def fixture(id: String, suffix: String): Json = file(fixtures.resolve(id + "." + suffix + ".json"))
  def snapshot(id: String): Snapshot = right(Decoding.snapshot(fixture(id, "request")))
  def profile: Json = file(profiles.resolve("racing.json"))
  def engine: Engine = new Engine(right(Profiles.load(profiles)))
  def update(json: Json, key: String, value: Json): Json = json match
    case Json.Obj(fields) => Json.Obj(fields.updated(key, value))
    case _ => throw new AssertionError("not an object")
  def remove(json: Json, key: String): Json = json match
    case Json.Obj(fields) => Json.Obj(fields - key)
    case _ => throw new AssertionError("not an object")
  def field(json: Json, key: String): Json = right(Json.read(right(Json.obj(json)), key)(Right(_)))
  def array(json: Json): Vector[Json] = right(Json.arr(json))
  def editEvent(json: Json, index: Int)(f: Json => Json): Json =
    val events = array(field(json, "events"))
    update(json, "events", Json.Arr(events.updated(index, f(events(index)))))
  def editData(json: Json, index: Int, key: String, value: Json): Json =
    editEvent(json, index)(event => update(event, "data", update(field(event, "data"), key, value)))
  def editMetric(json: Json, index: Int)(f: Json => Json): Json =
    val caps = array(field(json, "capabilities"))
    val metrics = array(field(caps.head, "metrics"))
    val first = update(caps.head, "metrics", Json.Arr(metrics.updated(index, f(metrics(index)))))
    update(json, "capabilities", Json.Arr(caps.updated(0, first)))
  def analyze(json: Json, profile: Json = TestData.profile): AnalysisResult =
    val p = right(Decoding.profile(profile))
    right(new Engine(Map(p.gameId -> p)).analyze(right(Decoding.snapshot(json))))
  def metric(result: AnalysisResult, id: String, subject: String = "p1"): Metric =
    result.metrics.find(m => m.id == id && m.subjectId == subject)
      .getOrElse(throw new AssertionError("missing metric " + id + "/" + subject))
