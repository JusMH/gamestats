import sbt._
import sbt.Keys._

// Build-only packaging: no plugin dependency, assembly or runtime sbt needed.
object DockerPackaging extends AutoPlugin {
  override def trigger = allRequirements
  object autoImport {
    val dockerStage = taskKey[File]("Stage the application JAR and runtime dependency JARs")
  }
  import autoImport._
  override lazy val projectSettings = Seq(
    dockerStage := {
      val application = (Compile / packageBin).value
      val dependencies = (Runtime / externalDependencyClasspath).value.map(_.data)
      require(dependencies.forall(f => f.isFile && f.getName.endsWith(".jar")),
        "Runtime dependencies must be JAR files")
      require(dependencies.map(_.getName).distinct.size == dependencies.size,
        "Runtime JAR names collide")
      val buildTarget = target.value.getCanonicalFile.toPath
      val stage = (target.value / "docker-stage").getCanonicalFile
      require(stage.toPath.startsWith(buildTarget) && stage.toPath != buildTarget,
        "Packaging directory must stay inside target")
      IO.delete(stage)
      IO.createDirectory(stage / "lib")
      IO.copyFile(application, stage / "app.jar")
      dependencies.foreach(file => IO.copyFile(file, stage / "lib" / file.getName))
      streams.value.log.info("Docker runtime staged in " + stage)
      stage
    }
  )
}