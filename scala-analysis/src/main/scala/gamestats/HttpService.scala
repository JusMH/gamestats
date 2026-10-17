package gamestats

import com.sun.net.httpserver.{HttpExchange, HttpServer}
import java.net.InetSocketAddress
import java.nio.file.Path
import java.util.concurrent.{ArrayBlockingQueue, ThreadPoolExecutor, TimeUnit}
import scala.util.control.NonFatal

case class ServerConfig(host: String = "127.0.0.1", port: Int = 9000,
  profilesDirectory: Path = Path.of("../docs/examples/profiles"))

object ServerConfig:
  def fromEnv(env: Map[String, String]): Checked[ServerConfig] =
    val host = env.getOrElse("SCALA_HOST", "127.0.0.1")
    val port = env.getOrElse("SCALA_PORT", "9000").toIntOption
    for
      _ <- Json.ensure(host.trim.nonEmpty, "invalid_host", "SCALA_HOST")
      number <- port.toRight(ValidationError("invalid_port", "SCALA_PORT"))
      _ <- Json.ensure(number >= 1 && number <= 65535, "invalid_port", "SCALA_PORT")
      directory <- try Right(Path.of(env.getOrElse("SCALA_PROFILES_DIR", "../docs/examples/profiles")))
        catch case NonFatal(_) => Left(ValidationError("invalid_path", "SCALA_PROFILES_DIR"))
    yield ServerConfig(host, number, directory)

final class RunningServer private[gamestats] (private val server: HttpServer,
  private val executor: ThreadPoolExecutor) extends AutoCloseable:
  def port: Int = server.getAddress.getPort
  def close(): Unit =
    server.stop(0)
    executor.shutdownNow()

object HttpService:
  val MaxBodyBytes = 8 * 1024 * 1024
  val MaxResponseBytes = 8 * 1024 * 1024

  def start(engine: Engine, config: ServerConfig): RunningServer =
    val server = HttpServer.create(new InetSocketAddress(config.host, config.port), 64)
    val executor = new ThreadPoolExecutor(4, 4, 0L, TimeUnit.MILLISECONDS,
      new ArrayBlockingQueue[Runnable](64), new ThreadPoolExecutor.CallerRunsPolicy())
    server.setExecutor(executor)
    server.createContext("/", exchange => handle(exchange, engine))
    server.start()
    new RunningServer(server, executor)

  private def error(code: String): Json = Json.Obj(Map("error" -> Json.Str(code)))
  private def send(exchange: HttpExchange, status: Int, body: Json): Unit =
    val encoded = Json.boundedBytes(body, MaxResponseBytes)
    val responseStatus = if encoded.isDefined then status else 503
    val bytes = encoded.getOrElse(Json.bytes(error("result_too_large")))
    exchange.getResponseHeaders.set("Content-Type", "application/json; charset=utf-8")
    if exchange.getRequestMethod == "HEAD" then exchange.sendResponseHeaders(responseStatus, -1)
    else
      exchange.sendResponseHeaders(responseStatus, bytes.length.toLong)
      exchange.getResponseBody.write(bytes)

  private def handle(exchange: HttpExchange, engine: Engine): Unit =
    try
      val path = exchange.getRequestURI.getPath
      val method = exchange.getRequestMethod
      val expectedMethod = path match
        case "/health" => Some("GET")
        case "/analyze" => Some("POST")
        case _ => None
      expectedMethod match
        case None => send(exchange, 404, error("not_found"))
        case Some(allowed) if method != allowed =>
          exchange.getResponseHeaders.set("Allow", allowed)
          send(exchange, 405, error("method_not_allowed"))
        case Some(_) if path == "/health" =>
          send(exchange, 200, Json.Obj(Map("status" -> Json.Str("ok"))))
        case Some(_) =>
          val bytes = exchange.getRequestBody.readNBytes(MaxBodyBytes + 1)
          if bytes.length > MaxBodyBytes then send(exchange, 413, error("request_too_large"))
          else
            val result = for
              json <- Json.parse(bytes)
              snapshot <- Decoding.snapshot(json)
              analyzed <- engine.analyze(snapshot)
            yield analyzed
            result match
              case Left(problem) =>
                val status = if problem.code == "capability_not_implemented" then 501 else 400
                send(exchange, status, error(problem.code))
              case Right(analyzed) =>
                send(exchange, 200, analyzed.json)
    catch
      case NonFatal(_) =>
        // Do not return exceptions, input bodies, profiles or internal addresses.
        System.err.println("Analysis HTTP request failed")
        try send(exchange, 500, error("internal_error"))
        catch case NonFatal(_) => ()
    finally exchange.close()
