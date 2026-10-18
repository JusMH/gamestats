# Ejecutar y revisar GameStats desde CMD

Esta guía corresponde a las fuentes del repositorio. Para instalar el paquete
publicado v1.0.1 sin compilar, seguir sus manuales y
[la distribución académica](academic-distribution.md).
Los ajustes de esta rama no modifican los ZIP ni las imágenes ya publicados.

## Carpeta y requisitos

Usar **CMD**. Docker Desktop debe estar iniciado con contenedores Linux.
La construcción desde fuentes requiere Internet la primera vez.
Para este procedimiento no hacen falta Go, Java, Scala, sbt ni Python.

Ubicar una copia de las fuentes en una ruta **sin tildes ni otros caracteres
no ASCII en todos sus niveles**, por ejemplo `%USERPROFILE%\gamestats-fran`.
En el equipo revisado, BuildKit falló en la ruta que contenía `Programación`
y `gamestats-revisión`, con el error de cabecera
`x-docker-expose-session-sharedkey` / `non-printable ASCII characters`.
La misma copia, verificada sin cambios, construyó correctamente en una ruta
sin tildes. Cambiar solo el nombre de la última carpeta no elimina las tildes
de sus carpetas superiores. No hace falta cambiar el código ni desactivar BuildKit.

## Construir e iniciar

Desde la raíz de esa copia:

```cmd
cd /d "%USERPROFILE%\gamestats-fran"
docker info
docker compose --project-name gamestats-fran --profile tools build
docker compose --project-name gamestats-fran up -d --wait
docker compose --project-name gamestats-fran ps
curl.exe -i http://127.0.0.1:8080/health
```

`docker info` debe mostrar el servidor sin errores de conexión.
Go y Scala deben aparecer como `healthy`, y la consulta de salud debe responder
HTTP 200 con `{"status":"ok"}`. Scala queda en la red interna de Docker;
no se consulta directamente desde el anfitrión por el puerto 9000.
La salud de Go no comprueba por sí sola que su análisis funcione.

Si el puerto 8080 está ocupado, antes del arranque definir:

```cmd
set "GO_PORT=8081"
```

En ese caso usar 8081 en las consultas del anfitrión. El simulador dentro de
Docker sigue usando la dirección interna configurada por Compose.

## Demostración y resultados

```cmd
docker compose --project-name gamestats-fran run --rm simulator --run-id prueba-01
echo %ERRORLEVEL%
docker compose --project-name gamestats-fran run --rm simulator --mode negative --run-id negativos-01
echo %ERRORLEVEL%
curl.exe -i http://127.0.0.1:8080/results/tic-tac-toe/prueba-01-ttt-win
curl.exe -i http://127.0.0.1:8080/results/snake/prueba-01-snake-positive
```

La demostración normal debe terminar con PASS y seis partidas verificadas:
50 eventos y 60 consultas, 48 abiertas y 12 cerradas. El modo negativo debe
aprobar sus tres comprobaciones. En ambos casos, ERRORLEVEL debe ser 0.
Gato debe identificar a alice como ganadora; Snake positivo debe mostrar
24 puntos, promedio 8 y una ráfaga para alice. Las respuestas finales indican
`state:closed`. Las consultas abiertas quedaron verificadas durante el simulador.

Para repetir sin reiniciar Go, usar otro identificador:

```cmd
docker compose --project-name gamestats-fran run --rm simulator --run-id prueba-02
```

Repetir `prueba-01` contra el mismo proceso Go debe fallar con seis rechazos
HTTP 409 por duplicados. La salida distinta de cero solo es esperada para esta
prueba deliberada; un timeout u otro error no demuestra duplicación correcta.

## Comprobar la recuperación de Scala

Manteniendo Go y sus partidas en ejecución:

```cmd
docker compose --project-name gamestats-fran stop scala-analysis
curl.exe -i http://127.0.0.1:8080/results/snake/prueba-01-snake-positive
curl.exe -i http://127.0.0.1:8080/matches/snake/prueba-01-snake-positive
docker compose --project-name gamestats-fran up -d --wait scala-analysis
curl.exe -i http://127.0.0.1:8080/results/snake/prueba-01-snake-positive
```

Durante la desconexión, resultados devuelve 502 o 504 según el fallo de red;
la consulta de partida sigue devolviendo 200 y conserva nueve eventos.
Tras arrancar Scala y esperar su salud, resultados vuelve a 200 con los
mismos valores. No se reingieren eventos ni se reinicia Go para recuperarlos.

## Informes con límite de tres

La rotación está implementada en las **fuentes posteriores a v1.0.1**; las
imágenes y el ZIP de v1.0.1 conservan su comportamiento publicado anterior.
Para probarla con Go instalado, mantener los servicios Docker y ejecutar en
otra ventana CMD, desde la carpeta `simulator`:

```cmd
cd /d "%USERPROFILE%\gamestats-fran\simulator"
go run ./cmd/simulator --run-id historial-01 --report evidence/reporte.json
go run ./cmd/simulator --run-id historial-02 --report evidence/reporte.json
go run ./cmd/simulator --run-id historial-03 --report evidence/reporte.json
go run ./cmd/simulator --run-id historial-04 --report evidence/reporte.json
dir evidence\reporte*.json
```

Si Go se publica en 8081, añadir `--go-url http://127.0.0.1:8081` a cada comando.
Al terminar deben existir solo `reporte.json` (historial-04), `reporte.1.json`
(historial-03) y `reporte.2.json` (historial-02). El más antiguo sale de esta
familia después de guardar el nuevo archivo correctamente. Los fallos también
generan evidencia y participan en la rotación: no se reservan tres éxitos.
El límite es por destino `--report`; usar un nombre estable evita crear familias
nuevas indefinidamente. Otros archivos y otras familias no se eliminan.
Si no se solicita `--report`, no se guardan informes.

En contenedores, `--report /reports/reporte.json` escribe dentro de la tarea.
Con `--rm` sus archivos desaparecen al terminar; no se promete conservación
entre tareas Docker sin un almacenamiento compartido. Esta guía usa el cliente
Go del anfitrión para conservar evidencia sin añadir volúmenes a Compose.

## Pruebas de desarrollo opcionales

Con Go y Python 3.10+ instalados, desde la raíz:

```cmd
cd go-ingestion
go test ./...
go vet ./...
cd ..\simulator
go test ./...
go vet ./...
cd ..
python -m unittest discover -s docker -p "test_*.py" -v
python -m unittest discover -s distribution -p "test_*.py" -v
python docs/fixtures/analysis-v1/check.py
```

Si Python se invoca mediante el launcher de Windows, sustituir `python` por
`py -3`. La lectura del JSON de Compose en las pruebas de infraestructura usa
UTF-8 explícitamente para conservar las rutas con tildes; no exige cambiar
la codificación global del sistema. `go test -race` requiere además cgo y GCC.

Para ejecutar Scala de forma nativa, con JDK 21 instalado, desde scala-analysis:

```cmd
set "JAVA_HOME=C:\Program Files\Eclipse Adoptium\jdk-21.0.8.9-hotspot"
"%JAVA_HOME%\bin\java.exe" -version
sbtw.cmd compile test
sbtw.cmd run
```

Ajustar JAVA_HOME a la instalación de cada equipo. El lanzador CMD verifica
el mismo SHA-256 de sbt 1.11.7 que el wrapper PowerShell, usa .tools/.cache
locales y no instala Java ni modifica variables globales.
No arrancar simultáneamente el Go nativo y el de Docker en el mismo puerto.

## Diagnóstico y cierre

```cmd
docker compose --project-name gamestats-fran logs --tail 100 go-ingestion scala-analysis
docker compose --project-name gamestats-fran down
```

El apagado elimina los contenedores y red de este proyecto. Go pierde sus
partidas en memoria; las imágenes y cachés de construcción permanecen.
No ejecutar limpieza global de Docker para repetir una demostración.

## Comprobar los nuevos límites

Desde la raíz de las fuentes actuales, en CMD:

```cmd
cd go-ingestion
go test -v ./internal/matches ./internal/httpapi -run "TestEventLimit|TestSnapshot|TestOversizedStart|TestCapacityHTTP"
cd ..
```

Los casos comprueban capacidad concurrente, rechazo sin mutación, reenvío del
ID rechazado y cierre con 10 000 eventos o exactamente 8 MiB serializados.
La salida debe terminar en PASS y ok, sin FAIL.

Para probar Go y Scala reales se necesita Python 3.10+ y Docker en ejecución.
Desde una copia de fuentes cuya ruta completa no contenga tildes ni caracteres
no ASCII, usar un nombre y una carpeta de evidencia nuevos en cada ejecución:

```cmd
py -3 docker\verify.py --project gs-revision-final-01 --run-id revision-final-01 --evidence docker\evidence\revision-final-01
```

El verificador construye desde estas fuentes, inicia servicios aislados y prueba
las seis partidas de demostración, negativos, duplicados y límites de ingestión.
El resumen debe indicar status: passed, ingestionLimits.openAndClosedAnalyzed:
true y resourcesReleased: true. Limpia únicamente sus propios contenedores.
No usar el ZIP v1.0.1 para probar estos cambios posteriores de la rama.
