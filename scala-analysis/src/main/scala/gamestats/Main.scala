package gamestats

object Main:
  def main(args: Array[String]): Unit =
    val prepared = for
      config <- ServerConfig.fromEnv(sys.env)
      profiles <- Profiles.load(config.profilesDirectory)
    yield (config, profiles)
    prepared match
      case Left(problem) =>
        System.err.println("Cannot start analysis service: " + problem.code + " at " + problem.path)
        sys.exit(1)
      case Right((config, profiles)) =>
        try
          val running = HttpService.start(new Engine(profiles), config)
          Runtime.getRuntime.addShutdownHook(new Thread(() => running.close()))
          println("GameStats Scala listening on " + config.host + ":" + running.port)
        catch
          case scala.util.control.NonFatal(_) =>
            System.err.println("Cannot bind analysis HTTP service")
            sys.exit(1)
