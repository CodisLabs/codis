import sbt.Keys._
import sbt._

object CodisScalaClientBuild extends Build {

  val basicSettings = Seq(
    organization := "com.wandoulabs.codis",
    version := "0.0.1-SNAPSHOT",
    scalaVersion := "2.10.5",
    publishMavenStyle := true,
    publishTo := {
      val nexus = "https://oss.sonatype.org/"
      if (isSnapshot.value) {
        Some("snapshots" at nexus + "content/repositories/snapshots")
      }
      else {
        Some("releases" at nexus + "service/local/staging/deploy/maven2")
      }
    },
    publishArtifact in Test := false,
    credentials += Credentials(Path.userHome / ".ivy2" / ".credentials"))

  lazy val root = Project(
    id = "root",
    base = file("."),
    settings = basicSettings ++ Seq(libraryDependencies ++= Dependencies.deps)
  )
}

object Dependencies {

  val AkkaVersion = "2.3.6"

  val basic = Seq(
    "io.spray" %% "spray-json" % "1.3.1"
  )

  val zk = Seq(
    "org.apache.curator" % "curator-recipes" % "2.7.0"
  )

  val redis = Seq(
    "com.etaty.rediscala" %% "rediscala" % "1.4.0"
  )

  val testKit = Seq(
    "com.typesafe.akka" %% "akka-testkit" % AkkaVersion % Test,
    "org.scalatest" %% "scalatest" % "2.2.4" % Test
  )
  
  val deps = basic ++ zk ++ redis ++ testKit
}
