package gamestats

import java.net.URI
import java.net.http.{HttpClient, HttpRequest, HttpResponse}
import java.time.Duration

class HttpSuite extends munit.FunSuite:
  import TestData.*
  private var server: RunningServer = _
  private val client = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build()
  override def beforeAll(): Unit =
    server = HttpService.start(engine, ServerConfig(port = 0))
  override def afterAll(): Unit =
    server.close()
    client.close()
  private def request(method: String, path: String, body: String = ""): HttpRequest =
    HttpRequest.newBuilder(URI.create("http://127.0.0.1:" + server.port + path))
      .timeout(Duration.ofSeconds(10))
      .header("Content-Type", "application/json")
      .method(method, HttpRequest.BodyPublishers.ofString(body)).build()
  private def send(method: String, path: String, body: String = ""): HttpResponse[String] =
    client.send(request(method, path, body), HttpResponse.BodyHandlers.ofString())

  supported.foreach { id =>
    test("real HTTP response equals complete fixture: " + id) {
      val body = Json.text(fixture(id, "request"))
      val response = send("POST", "/analyze", body)
      assertEquals(response.statusCode(), 200)
      assert(response.headers().firstValue("Content-Type").orElse("").startsWith("application/json"))
      assertEquals(parse(response.body()), fixture(id, "expected"))
      assertEquals(parse(send("POST", "/analyze", body).body()), fixture(id, "expected"))
    }
  }
  test("health, routes and method Allow headers") {
    assertEquals(send("GET", "/health").statusCode(), 200)
    assertEquals(parse(send("GET", "/health").body()), parse("""{"status":"ok"}"""))
    assertEquals(send("GET", "/health/extra").statusCode(), 404)
    assertEquals(send("POST", "/analyze/extra", "{}").statusCode(), 404)
    Vector(("GET", "/analyze", "POST"), ("HEAD", "/analyze", "POST"),
      ("POST", "/health", "GET"), ("HEAD", "/health", "GET")).foreach { (method, path, allowed) =>
      val response = send(method, path)
      assertEquals(response.statusCode(), 405)
      assertEquals(response.headers().firstValue("Allow").orElse(""), allowed)
      if method == "HEAD" then assertEquals(response.body(), "")
    }
  }
  test("HTTP rejects invalid requests without leaking internal details") {
    Vector("{", "[]", "null", """{"secret":"private-token"}""",
      Json.text(update(fixture("race-final", "request"), "events", Json.Arr(Vector.empty)))).foreach { body =>
      val response = send("POST", "/analyze", body)
      assertEquals(response.statusCode(), 400)
      assert(!response.body().contains("private-token"))
      assert(!response.body().contains("Exception"))
      assert(right(Json.obj(parse(response.body()))).contains("error"))
    }
  }
  test("HTTP marks pending capabilities as unimplemented, never empty success") {
    Vector("snake-positive", "ttt-win").foreach { id =>
      val response = send("POST", "/analyze", Json.text(fixture(id, "request")))
      assertEquals(response.statusCode(), 501)
      assertEquals(parse(response.body()), parse("""{"error":"capability_not_implemented"}"""))
    }
  }
  test("bad roster is HTTP 400, not a fabricated player") {
    val body = editData(fixture("race-final", "request"), 0, "players", Json.Arr(Vector.empty))
    assertEquals(send("POST", "/analyze", Json.text(body)).statusCode(), 400)
  }
  test("body limit is enforced before parsing") {
    val response = send("POST", "/analyze", " " * (HttpService.MaxBodyBytes + 1))
    assertEquals(response.statusCode(), 413)
  }
  test("different matches are analyzed concurrently with isolated results") {
    val base = fixture("race-final", "request")
    val futures = (0 until 16).map { i =>
      val matchId = "concurrent-" + i
      val body = update(update(base, "matchId", Json.Str(matchId)), "events",
        Json.Arr(array(field(base, "events")).map(e => update(e, "matchId", Json.Str(matchId)))))
      client.sendAsync(request("POST", "/analyze", Json.text(body)),
        HttpResponse.BodyHandlers.ofString())
    }
    futures.zipWithIndex.foreach { (future, i) =>
      val response = future.get(15, java.util.concurrent.TimeUnit.SECONDS)
      assertEquals(response.statusCode(), 200)
      val json = parse(response.body())
      assertEquals(field(json, "matchId"), Json.Str("concurrent-" + i))
      val metrics = array(field(json, "metrics"))
      val laps = metrics.find(m => field(m, "id") == Json.Str("laps.player") &&
        field(m, "subjectId") == Json.Str("p1")).get
      assertEquals(field(laps, "value"), Json.IntNum(3))
    }
  }
