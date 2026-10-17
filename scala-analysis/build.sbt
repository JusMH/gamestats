ThisBuild / scalaVersion := "3.3.7"
ThisBuild / organization := "gamestats"
ThisBuild / version := "0.1.0"

libraryDependencies ++= Seq(
  "com.fasterxml.jackson.core" % "jackson-databind" % "2.20.1",
  "org.scalameta" %% "munit" % "1.2.1" % Test
)
scalacOptions ++= Seq("-deprecation", "-feature", "-unchecked", "-release:21")
Compile / mainClass := Some("gamestats.Main")
Compile / run / fork := true
Test / fork := true
Test / parallelExecution := false
