# GameStats Scala — implementación 4B

Servicio real Scala 3 con POST /analyze y GET /health. Ejecuta perfiles
declarativos con event-metrics/v1 (count/sum/avg), duration/v1,
temporal-window/v1 y el analizador tic-tac-toe-line/v1.
No hay lógica por videojuego en el ejecutor, estado acumulado entre consultas
ni base de datos dentro del motor. El simulador es externo; CI y distribución
ya existen en el repositorio. Docker se documenta al final.

## Requisitos y versiones

| Componente | Versión fijada | Motivo |
| --- | --- | --- |
| JDK | 21; probado con Temurin 21.0.12.1 | Incluye HttpServer, HttpClient y java.time; sin biblioteca HTTP externa. |
| Scala | 3.3.7 LTS | Modelos inmutables, enums y compatibilidad con JDK 21. |
| sbt | 1.11.7 | Compatible con JDK 21; lanzador local reproducible. |
| Jackson databind | 2.20.1 | JSON estricto, límites y BigDecimal sin convertir datos a Double. |
| MUnit | 1.2.1, solo pruebas | Pruebas habituales Scala 3; motor puro y HTTP real. |

Jackson es una biblioteca Java interoperable con Scala, sin plugin ni
módulo de reflexión Scala; trae jackson-core y jackson-annotations de forma
transitiva. No se usa evaluación de código desde perfiles.
CI selecciona Temurin `21` y resuelve su actualización disponible; no fija
21.0.12.1. La imagen Docker sí fija JDK/JRE 21.0.12+8 y su digest.
Las versiones del proyecto son fijadas y comprobadas, no una afirmación de que sean
las más recientes. Referencias: [compatibilidad JDK/Scala/sbt](https://docs.scala-lang.org/overviews/jdk-compatibility/overview.html),
[sbt 1.11.7](https://github.com/sbt/sbt/releases/tag/v1.11.7),
[Jackson 2.20.1](https://github.com/FasterXML/jackson/wiki/Jackson-Release-2.20.1)
y [MUnit 1.2.1](https://github.com/scalameta/munit/releases/tag/v1.2.1).

## Preparación reproducible en Windows

Desde PowerShell, en esta carpeta:

~~~powershell
java -version
.\sbtw.ps1 compile test
.\sbtw.ps1 run
~~~

No se necesita sbt ni Scala instalado globalmente, ni un IDE.
sbtw.ps1 descarga el lanzador sbt fijado desde Maven Central a .tools,
comprueba su SHA-256 y dirige boot, global-base, Ivy y Coursier a .cache.
Los dos directorios están ignorados por Git. No instala software, no usa
administrador ni modifica variables o configuraciones globales.
La primera ejecución necesita acceso a Maven Central; las siguientes
reutilizan las dependencias locales. No se versionan JARs ni cachés.

Si ya se dispone de sbt y JDK 21, también funcionan:

~~~sh
sbt compile test
sbt run
~~~

El JDK ya debe estar instalado. El wrapper no instala Java ni lo actualiza.
Se compila para Java 21. build.sbt y project/build.properties fijan las versiones.

## Configuración

| Variable | Valor por defecto |
| --- | --- |
| SCALA_HOST | 127.0.0.1 |
| SCALA_PORT | 9000 |
| SCALA_PROFILES_DIR | ../docs/examples/profiles, relativo al directorio de ejecución |

~~~powershell
$env:SCALA_HOST = '127.0.0.1'
$env:SCALA_PORT = '9000'
$env:SCALA_PROFILES_DIR = '../docs/examples/profiles'
.\sbtw.ps1 run
~~~

Arrancar desde scala-analysis; Ctrl+C detiene el proceso.
Para ejecutar desde otra carpeta se recomienda una ruta absoluta de perfiles.
El puerto debe ser un entero 1..65535; un host vacío o configuración inválida
impide arrancar. Una carpeta inexistente/vacía, perfiles inválidos o gameId
duplicados también impiden arrancar. Los perfiles se cargan una sola vez.

## Endpoints y ejemplo real

~~~sh
curl http://localhost:9000/health
curl -X POST http://localhost:9000/analyze -H "Content-Type: application/json" --data-binary "@../docs/fixtures/analysis-v1/race-live-2.request.json"
~~~

En Windows puede usarse curl.exe. En Postman elegir POST, URL
http://localhost:9000/analyze, Body → raw → JSON, con el contenido del fixture.
No enviar los metadatos del manifiesto; el fixture request es el cuerpo exacto.

El ejemplo real race-live-2 produce, entre otras filas:
- laps.player/p1 = 2, provisional.
- lap_time.total.player/p1 = 22000 ms, provisional.
- lap_time.avg.player/p1 = 11000 ms, provisional.
- duration.match/race-live = null, unavailable/awaiting_finish.
- p2 tiene conteo/suma 0 y media unavailable/no_samples.

La respuesta completa se compara automáticamente con
[race-live-2.expected.json](../docs/fixtures/analysis-v1/race-live-2.expected.json).
[El caso final](../docs/fixtures/analysis-v1/race-final.expected.json) produce
3 vueltas, 30000 ms totales y 10000 ms de media para p1, ahora final;
la duración de partida es 32 s. Consultar de nuevo no acumula contadores.

## Integración con Go existente

Iniciar Scala y después, desde otra terminal:

~~~powershell
$env:SCALA_SERVICE_URL = 'http://localhost:9000'
cd ..\go-ingestion
go run ./cmd/server
~~~

Enviar individualmente y en orden los eventos del fixture por POST
http://localhost:8080/events. Tras cada prefijo relevante consultar
GET http://localhost:8080/results/racing/race-live.
Primero MATCH_STARTED, después vueltas y finalmente MATCH_FINISHED.
No reingerir los prefijos completos: los eventId repetidos se rechazan.

Go toma un snapshot consistente y hace POST /analyze por cada GET.
No cambia el código Go, no se cierra para consultar y no hay push/caché.
Una respuesta corresponde al snapshot enviado, aunque lleguen más eventos.

## Arquitectura

~~~text
src/main/scala/gamestats/
  Json.scala         JSON inmutable, límites, lectura/escritura estrictas
  Model.scala        Eventos, snapshots, perfiles, métricas, patrones y errores
  Decoding.scala     Validación tipada y carga de perfiles
  Engine.scala       Interpretación funcional, composición y capacidades
  TemporalWindow.scala Ventanas genéricas con consumo greedy
  TicTacToeLine.scala Analizador especializado y registro analyzerId
  HttpService.scala  HTTP JDK, concurrencia y códigos
  Main.scala         Variables, carga inicial y ciclo de vida
src/test/scala/gamestats/
  TestData.scala
  EngineSuite.scala
  ValidationSuite.scala
  HttpSuite.scala
  PatternsSuite.scala
~~~

El motor se invoca sin servidor con Engine.analyze(snapshot), que devuelve
Either[ValidationError, AnalysisResult]. Los modelos JSON son inmutables;
los nodos mutables de Jackson permanecen en la frontera de serialización.
Los casos de uso públicos decodifican/validan el snapshot antes de analizar.

Los perfiles son un DSL de datos tipado: definen IDs versionados, eventos,
campos JSON Pointer, operaciones, ámbitos y presentación. Un registro
resuelve capacidades; no existe switch/if por gameId ni scripts evaluados.
filter selecciona, groupBy agrupa jugadores, map extrae valores validados,
foldLeft acumula sumas exactas y validaciones, flatMap compone resultados.
Las ventanas ordenan por UTC e índice original; el tablero usa foldLeft
con estado inmutable y se selecciona por analyzerId. No se descartan
muestras numéricas incorrectas silenciosamente.

Para otro juego basta añadir un perfil basado en las capacidades disponibles.
Las pruebas agregan un gameId y una métrica sin cambiar el ejecutor.
No se reescribieron los perfiles ni fixtures aprobados.

## Capacidades y analizadores disponibles

Los tres perfiles aprobados se ejecutan completos, sin reescribirlos.
temporal-window/v1 usa N y duración configurados, límite inclusivo y consumo
sin solapamientos. tic-tac-toe-line/v1 reconstruye el tablero, verifica sus
datos y detecta la primera línea según el desempate documentado.

Snake y Gato responden HTTP 200 con métricas y patrones completos.
Se eliminó la política transitoria 501. Una configuración con IDs o versiones
desconocidos sigue impidiendo arrancar; no se ignoran capacidades.

source distingue templateId/analyzerId; board.result admite valor estructurado
y las ocurrencias de línea incluyen details. Los valores inválidos o no
disponibles siguen usando null y reason. El contrato exterior no cambia.
[Arquitectura, alcance y verificación de 4B](../docs/scala-4b.md).

## Errores, precisión y límites

| Situación | HTTP Scala |
| --- | --- |
| Snapshot válido, incluso juego sin perfil | 200 |
| Campo numérico del dominio inválido | 200, métricas afectadas invalid/reason |
| JSON/snapshot/roster inválido | 400, error público sin trazas |
| Método no permitido | 405 y Allow |
| Ruta desconocida | 404 |
| Petición mayor de 8 MiB | 413 |
| Respuesta superior a 8 MiB | 503 |
| Fallo interno | 500 genérico |

Sin perfil: metrics=[] y patterns=[], sin estadísticas inventadas.
Sin muestras: count/sum=0, avg=null/unavailable. Un número incorrecto no se
omite ni se transforma en cero. Todos los estados y procedencias siguen
[el contrato interior](../docs/metrics-and-patterns.md).

Jackson conserva números enteros/decimales exactos. Las sumas utilizan
java.math.BigDecimal.add sin contexto de redondeo intermedio; avg y segundos
fraccionarios se preparan una sola vez a seis decimales HALF_EVEN en la
frontera de resultado, sin métricas que consuman valores ya redondeados.
No hay pérdida por Double. Los timestamps se comparan como instantes;
se aceptan zonas RFC3339 hasta +/-23:59 y se truncan fracciones posteriores
a nanosegundos para mantener compatibilidad con Go.

Límites: 10000 eventos/snapshot, 1000 jugadores, 128 perfiles,
128 instancias/perfil, 1000 IDs de salida/perfil y hasta 20000 filas
estimadas por análisis. Cada perfil admite hasta 1 MiB, cada lista de
métricas hasta 256. JSON: profundidad 64, cadenas hasta 65536 caracteres,
tokens numéricos hasta 128 caracteres y escala decimal absoluta hasta 1000.
La salida HTTP se serializa con límite de memoria de bytes.
Hay cuatro workers HTTP y cola de 64 con contrapresión; no se guarda estado.
No hay autenticación, persistencia ni tratamiento avanzado de clientes lentos.
Son límites operativos explícitos, no nuevas reglas del juego.

## Pruebas y fixtures

~~~powershell
.\sbtw.ps1 compile test
~~~

**143 pruebas Scala aprobadas**. Los 13 fixtures se comparan completos en
el motor puro, mediante HTTP real y en consultas concurrentes. Las pruebas
anteriores de configuración, precisión, duración y composición se conservan.

| Fixtures completos comparados con expected | Estado 4B |
| --- | --- |
| race-start, race-live-2, race-live-3, race-final | Motor, HTTP y Go → Scala |
| unknown-game | Motor, HTTP y Go → Scala |
| ttt-win, ttt-draw, ttt-open | Motor, HTTP y Go → Scala |
| snake-positive, snake-negative, snake-invalid-number, snake-missing-points, snake-tied | Motor, HTTP y Go → Scala |

Se verifican valores, sujetos, estados, unidades, procedencia, versión,
basis, reasons, occurrences y detalles de tablero. No se modificaron los
expected. Las suites cubren un nanosegundo fuera del límite, 9000 candidatos,
solapamientos, zonas equivalentes, eventos tardíos, errores independientes,
las ocho líneas ganadoras, desempates, casillas/turnos/símbolos inválidos,
ganadores ambiguos y estados parciales.

### Integración reproducible con procesos reales

1. Iniciar Scala como se indica arriba.
2. Iniciar un proceso Go nuevo, sin partidas previas, con
   SCALA_SERVICE_URL=http://localhost:9000.
3. Desde la raíz del repositorio, con Python 3.10+:

~~~sh
python scala-analysis/scripts/check-go.py --evidence-dir scala-analysis/.tools/integration-4b
~~~

La prueba no es un simulador: envía únicamente los eventos aprobados de los
fixtures y compara respuestas completas. No genera acontecimientos nuevos.
No usa mocks, no reescribe resultados y no borra partidas existentes.
Un segundo intento requiere reiniciar Go porque los IDs ya estarán aceptados.

Resultado ejecutado: **61 eventos**, **10 partidas**, **13 fixtures**,
**26 respuestas secuenciales** y **20 concurrentes**, todas correctas.
Los prefijos de carreras se consultan antes de avanzar; no se reingieren.
Las respuestas y summary.json se guardan en evidence-dir si se solicita.
La verificación de esta entrega detuvo sus procesos propios al finalizar.

También pasaron go test ./..., go vet ./... y el verificador Python de
la Etapa 1 (30 JSON, 13 pares y nueve mutaciones inválidas de perfiles).
El verificador documental sigue siendo un oráculo de diseño; no sustituye
las pruebas Scala ni la integración real.

## Alcance y siguiente tarea

El tablero comprueba evidencia de una línea, no alternancia, X primero ni
detención inmediata tras ganar. Un cierre incompleto no inventa un empate:
resultado unavailable/insufficient_evidence, con cero líneas observadas.
No se valida física de Snake ni legalidad semántica de vueltas.

El simulador multijuego de la implementación 5 ya está disponible; la ejecución
Docker de la implementación 6 se describe a continuación. CI y la distribución
v1.0.0 ya se verificaron; ver [estado vigente](../docs/academic-audit.md).
Históricamente, durante 4B no se implementaron simulador, Docker, CI/CD, deployment, base de datos,
WebSockets/SSE ni reglas universales dentro de 4B.


## Empaquetado y ejecución Docker

Desde la raíz: docker compose up -d --build. Compose inicia Scala en
0.0.0.0:9000, sin publicar el puerto al anfitrión, y Go usa su nombre DNS.
Los perfiles aprobados se incluyen en /app/profiles; la validación existente
impide arrancar con perfiles ausentes, vacíos o inválidos.

La construcción usa JDK 21.0.12+8 y el lanzador sbt 1.11.7 verificado por
SHA-256. El nuevo task dockerStage (project/DockerPackaging.scala) empaqueta
app.jar y cinco JAR de runtime, excluyendo MUnit. La imagen final usa JRE 21,
usuario 10001:10001 y no contiene sbt, fuentes ni cachés. No cambian motor,
versiones Scala/dependencias ni perfiles.

Comprobación local opcional, con JDK 21, desde scala-analysis:

~~~powershell
.\sbtw.ps1 dockerStage compile test
cd ..
python docker/check-stage.py
~~~

Esta prueba verifica el artefacto en el anfitrión; no equivale a ejecutar
un contenedor. [Guía Docker y resultados de integración](../docs/docker.md).
