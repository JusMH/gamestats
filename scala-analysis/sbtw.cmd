@echo off
setlocal EnableExtensions DisableDelayedExpansion
rem Project-local launcher and caches. No installation or persistent env changes.
pushd "%~dp0" || exit /b 1
set "SBT_VERSION=1.11.7"
set "SBT_HASH=F92A2095AC75008764FE3B2B793FFE624C4FBEF5BFD9B0022E4BC2DAF668C651"
set "SBT_LAUNCHER=%CD%\.tools\sbt-launch-%SBT_VERSION%.jar"
set "SBT_JAVA=java.exe"
if defined JAVA_HOME set "SBT_JAVA=%JAVA_HOME%\bin\java.exe"
"%SBT_JAVA%" -version >nul 2>&1
if errorlevel 1 (
  echo ERROR: Java is unavailable. Set JAVA_HOME to a JDK 21 installation.
  goto failed
)

if not exist "%SBT_LAUNCHER%" (
  if not exist ".tools" mkdir ".tools"
  if not exist ".tools" goto failed
  echo Downloading sbt %SBT_VERSION% from Maven Central...
  curl.exe --fail --location --silent --show-error --connect-timeout 15 --max-time 180 --output "%SBT_LAUNCHER%.download" "https://repo.maven.apache.org/maven2/org/scala-sbt/sbt-launch/%SBT_VERSION%/sbt-launch-%SBT_VERSION%.jar"
  if errorlevel 1 goto failed
  certutil.exe -hashfile "%SBT_LAUNCHER%.download" SHA256 | findstr.exe /i /x /c:"%SBT_HASH%" >nul
  if errorlevel 1 goto checksumFailed
  move /y "%SBT_LAUNCHER%.download" "%SBT_LAUNCHER%" >nul
  if errorlevel 1 goto failed
)

certutil.exe -hashfile "%SBT_LAUNCHER%" SHA256 | findstr.exe /i /x /c:"%SBT_HASH%" >nul
if errorlevel 1 goto checksumFailed
set "COURSIER_CACHE=%CD%\.cache\coursier"
"%SBT_JAVA%" -Xmx1g -Dsbt.supershell=false -Dsbt.server.autostart=false -Dsbt.override.build.repos=true "-Dsbt.repository.config=%CD%/project/repositories" "-Dsbt.global.base=%CD%/.cache/global" "-Dsbt.boot.directory=%CD%/.cache/boot" "-Dsbt.ivy.home=%CD%/.cache/ivy" -jar "%SBT_LAUNCHER%" %*
set "SBT_EXIT=%ERRORLEVEL%"
popd
exit /b %SBT_EXIT%

:checksumFailed
echo ERROR: sbt launcher SHA-256 verification failed. The launcher was not executed.
:failed
popd
exit /b 1
