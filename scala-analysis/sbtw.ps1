# Project-local launcher and caches. No installation or persistent env changes.
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Commands)
$ErrorActionPreference='Stop'
Push-Location $PSScriptRoot
$taskPreviousCache=$env:COURSIER_CACHE
try {
 $taskVersion='1.11.7'
 $taskLauncher=Join-Path $PSScriptRoot ".tools/sbt-launch-$taskVersion.jar"
 $taskHash='F92A2095AC75008764FE3B2B793FFE624C4FBEF5BFD9B0022E4BC2DAF668C651'
 if (!(Test-Path -LiteralPath $taskLauncher)) {
  New-Item -ItemType Directory -Force (Split-Path $taskLauncher) | Out-Null
  Invoke-WebRequest "https://repo.maven.apache.org/maven2/org/scala-sbt/sbt-launch/$taskVersion/sbt-launch-$taskVersion.jar" -OutFile $taskLauncher
 }
 if ((Get-FileHash $taskLauncher -Algorithm SHA256).Hash -ne $taskHash) { throw 'Invalid sbt launcher checksum' }
 $env:COURSIER_CACHE=Join-Path $PSScriptRoot '.cache/coursier'
 & java '-Xmx1g' '-Dsbt.supershell=false' '-Dsbt.server.autostart=false' '-Dsbt.override.build.repos=true' "-Dsbt.repository.config=$PSScriptRoot/project/repositories" "-Dsbt.global.base=$PSScriptRoot/.cache/global" "-Dsbt.boot.directory=$PSScriptRoot/.cache/boot" "-Dsbt.ivy.home=$PSScriptRoot/.cache/ivy" -jar $taskLauncher @Commands
 $taskExit=$LASTEXITCODE
} finally {
 $env:COURSIER_CACHE=$taskPreviousCache
 Pop-Location
}
exit $taskExit
