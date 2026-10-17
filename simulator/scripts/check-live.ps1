# Runs only project-local builds and owned processes. No global configuration.
param(
 [string]$RunID='live5',
 [ValidateRange(1,65535)][int]$ScalaPort=9000
)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$taskSimulator=Join-Path $taskRoot 'simulator'
$taskEvidence=Join-Path $taskSimulator 'evidence/live'
$taskBin=Join-Path $taskSimulator 'bin'
New-Item -ItemType Directory -Force $taskEvidence,$taskBin | Out-Null
function Assert-FreePort([int]$Port) {
 $taskListener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Any,$Port)
 try { $taskListener.Start() } catch { throw "Port $Port already in use; no existing process will be stopped." }
 finally { $taskListener.Stop() }
}
function Wait-Health([string]$URL,$Process) {
 $taskDeadline=[DateTime]::UtcNow.AddSeconds(30)
 do {
  if ($Process.HasExited) { throw "Service exited: $URL; inspect logs in $taskEvidence" }
  try { $taskResponse=Invoke-RestMethod -Uri $URL -TimeoutSec 1; return } catch {}
  Start-Sleep -Milliseconds 100
 } while ([DateTime]::UtcNow -lt $taskDeadline)
 throw "Service not ready: $URL"
}
function Invoke-Simulation([string]$Name,[string[]]$Arguments,[int]$ExpectedExit=0) {
 $taskProcess=Start-Process -FilePath (Join-Path $taskBin 'simulator.exe') -ArgumentList $Arguments -WorkingDirectory $taskSimulator -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskEvidence "$Name.log") -RedirectStandardError (Join-Path $taskEvidence "$Name.err.log")
 try {
  if (!$taskProcess.WaitForExit(120000)) { throw "Simulator deadline: $Name" }
  if ($taskProcess.ExitCode -ne $ExpectedExit) { throw "$Name exit=$($taskProcess.ExitCode), expected=$ExpectedExit; inspect $taskEvidence" }
 } finally { if (!$taskProcess.HasExited) { Stop-Process -Id $taskProcess.Id } }
 Get-Content -LiteralPath (Join-Path $taskEvidence "$Name.log") | Select-Object -Last 9
}
Assert-FreePort 8080
Assert-FreePort $ScalaPort
if ($ScalaPort -eq 8080) { throw 'Scala must use a port different from Go (8080).' }
$taskPrevious=@{}
foreach ($taskName in @('SCALA_HOST','SCALA_PORT','SCALA_PROFILES_DIR','SCALA_SERVICE_URL')) { $taskPrevious[$taskName]=[Environment]::GetEnvironmentVariable($taskName,'Process') }
$taskScalaProcess=$null
$taskGoProcess=$null
try {
 Push-Location (Join-Path $taskRoot 'scala-analysis')
 try {
  $taskBuild=& .\sbtw.ps1 compile 'export Compile / fullClasspath'
  if ($LASTEXITCODE -ne 0) { throw 'Scala build failed' }
  $taskBuild | Set-Content -LiteralPath (Join-Path $taskEvidence 'scala-build.log') -Encoding utf8
  $taskClasspath=@($taskBuild | Where-Object { $_ -match 'scala-3\..*classes;' })[-1]
  if (!$taskClasspath) { throw 'No classpath exported by sbt' }
 } finally { Pop-Location }
 Push-Location (Join-Path $taskRoot 'go-ingestion')
 try { & go build -o (Join-Path $taskBin 'go-ingestion.exe') ./cmd/server; if ($LASTEXITCODE -ne 0) { throw 'Go build failed' } } finally { Pop-Location }
 Push-Location $taskSimulator
 try { & go build -o (Join-Path $taskBin 'simulator.exe') ./cmd/simulator; if ($LASTEXITCODE -ne 0) { throw 'Simulator build failed' } } finally { Pop-Location }
 $env:SCALA_HOST='127.0.0.1'
 $env:SCALA_PORT="$ScalaPort"
 $env:SCALA_PROFILES_DIR=Join-Path $taskRoot 'docs/examples/profiles'
 $env:SCALA_SERVICE_URL="http://127.0.0.1:$ScalaPort"
 $taskScalaProcess=Start-Process -FilePath (Get-Command java).Source -ArgumentList @('-cp',('"'+$taskClasspath+'"'),'gamestats.Main') -WorkingDirectory (Join-Path $taskRoot 'scala-analysis') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskEvidence 'scala.log') -RedirectStandardError (Join-Path $taskEvidence 'scala.err.log')
 Wait-Health "$env:SCALA_SERVICE_URL/health" $taskScalaProcess
 $taskGoProcess=Start-Process -FilePath (Join-Path $taskBin 'go-ingestion.exe') -WorkingDirectory $taskRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskEvidence 'go.log') -RedirectStandardError (Join-Path $taskEvidence 'go.err.log')
 Wait-Health 'http://127.0.0.1:8080/health' $taskGoProcess
 Invoke-Simulation 'demo' @('--run-id',$RunID,'--report','evidence/live/demo.json')
 Invoke-Simulation 'duplicate' @('--run-id',$RunID,'--report','evidence/live/duplicate.json') 1
 Invoke-Simulation 'negative' @('--run-id',$RunID,'--mode','negative','--report','evidence/live/negative.json')
 $taskDemo=Get-Content -Raw -LiteralPath (Join-Path $taskEvidence 'demo.json') | ConvertFrom-Json
 $taskDuplicate=Get-Content -Raw -LiteralPath (Join-Path $taskEvidence 'duplicate.json') | ConvertFrom-Json
 $taskNegative=Get-Content -Raw -LiteralPath (Join-Path $taskEvidence 'negative.json') | ConvertFrom-Json
 if (@($taskDemo.matches | Where-Object { !$_.verified -or $_.error }).Count -ne 0) { throw 'Demo has unverified matches' }
 if ($taskDemo.peakRequests -lt 2) { throw 'No overlapping HTTP activity observed' }
 if (@($taskDuplicate.matches | Where-Object { $_.accepted -ne 0 -or $_.error -notmatch 'HTTP 409' }).Count -ne 0) { throw 'Duplicate policy not demonstrated' }
 if (@($taskNegative.checks | Where-Object { !$_.passed }).Count -ne 0) { throw 'Negative cases failed' }
 $taskSummary=[ordered]@{
  runId=$RunID
  matches=@($taskDemo.matches).Count
  eventsAccepted=[int]($taskDemo.matches | Measure-Object -Property accepted -Sum).Sum
  resultsChecked=[int]($taskDemo.matches | Measure-Object -Property queries -Sum).Sum
  openResults=@($taskDemo.matches.results | Where-Object state -eq 'open').Count
  closedResults=@($taskDemo.matches.results | Where-Object state -eq 'closed').Count
  peakRequests=$taskDemo.peakRequests
  negativeChecks=@($taskNegative.checks).Count
  duplicateMatchesRejected=@($taskDuplicate.matches).Count
  goProcessId=$taskGoProcess.Id
  scalaProcessId=$taskScalaProcess.Id
 }
 $taskSummary | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $taskEvidence 'summary.json') -Encoding utf8
 $taskSummary | ConvertTo-Json
} finally {
 foreach ($taskOwned in @($taskGoProcess,$taskScalaProcess)) { if ($null -ne $taskOwned -and !$taskOwned.HasExited) { Stop-Process -Id $taskOwned.Id; $taskOwned.WaitForExit() } }
 foreach ($taskName in $taskPrevious.Keys) { [Environment]::SetEnvironmentVariable($taskName,$taskPrevious[$taskName],'Process') }
}