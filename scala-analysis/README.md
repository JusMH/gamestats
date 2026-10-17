# GameStats Scala — implementación 4A

Servicio real Scala 3 con POST /analyze y GET /health. Ejecuta perfiles
declarativos con event-metrics/v1 (count/sum/avg) y duration/v1.
No hay lógica por videojuego en el ejecutor, estado entre consultas,
simulador, Docker, CI/CD ni base de datos.

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
Las versiones son fijadas y comprobadas, no una afirmación de que sean
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
  HttpService.scala  HTTP JDK, concurrencia y códigos
  Main.scala         Variables, carga inicial y ciclo de vida
src/test/scala/gamestats/
  TestData.scala
  EngineSuite.scala
  ValidationSuite.scala
  HttpSuite.scala
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
collect identifica tipos de capacidades pendientes, sin descartar muestras
numéricas incorrectas.

Para otro juego basta añadir un perfil basado en las capacidades disponibles.
Las pruebas agregan un gameId y una métrica sin cambiar el ejecutor.
No se reescribieron los perfiles ni fixtures aprobados.

## Política explícita para 4B

Se cargan y validan los tres perfiles aprobados sin alterarlos.
temporal-window/v1 y tic-tac-toe-line/v1 se reconocen como contratos
pendientes, pero NO se ejecutan. Si el perfil los incluye, se rechaza
todo el análisis con HTTP 501 y {"error":"capability_not_implemented"}.
No se omiten capacidades silenciosamente ni se devuelve un 200 parcial.

Por ello los perfiles completos Snake y Gato todavía no son ejecutables.
La capacidad genérica inicio-muerte sí está implementada y probada en
aislamiento; eso no significa que el perfil completo Snake haya pasado.
Un ID de plantilla/analizador verdaderamente desconocido impide arrancar,
como exige la Etapa 1. El tratamiento de contratos conocidos pendientes es
una política transitoria documentada en [scala-4a.md](../docs/scala-4a.md).
Go transforma 501 de Scala en 502, manteniendo su comportamiento actual.

## Errores, precisión y límites

| Situación | HTTP Scala |
| --- | --- |
| Snapshot válido, incluso juego sin perfil | 200 |
| Campo numérico del dominio inválido | 200, métricas afectadas invalid/reason |
| JSON/snapshot/roster inválido | 400, error público sin trazas |
| Método no permitido | 405 y Allow |
| Ruta desconocida | 404 |
| Petición mayor de 8 MiB | 413 |
| Capacidad pendiente del perfil | 501 |
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

Las suites prueban el motor puro y peticiones HTTP reales en puerto efímero,
incluyendo concurrencia, repetición, números grandes, precisión, datos
incorrectos, perfiles inválidos, duración y configuración del servidor.

| Fixtures completos comparados con expected | Estado 4A |
| --- | --- |
| race-start, race-live-2, race-live-3, race-final | Ejecutados, motor y HTTP |
| unknown-game | Ejecutado, motor y HTTP |
| ttt-win, ttt-draw, ttt-open | Pendientes de 4B |
| snake-positive, snake-negative, snake-invalid-number, snake-missing-points, snake-tied | Pendientes de 4B |

Las pruebas de los ocho casos pendientes comprueban su rechazo explícito;
NO comparan una respuesta analítica completa ni afirman que sus reglas pasen.
El verificador Python de la Etapa 1 sigue siendo un oráculo de diseño,
no sustituye estas pruebas Scala.

## Siguiente etapa

Registrar temporal-window/v1 con orden (timestamp, índice de aceptación),
límite inclusivo y consumo sin solapamientos; implementar después el
analizador de tablero Gato y su procedencia analyzerId.
Ejecutar los ocho fixtures pendientes completos, además de eventos simultáneos,
tardíos, casillas inválidas, doble ganador y snapshots parciales.
No añadir simulador, Docker, UI, persistencia ni un lenguaje universal dentro de 4B.

## Evidencia de verificación de 4A

En el entorno de esta entrega: compilación correcta y **91 pruebas MUnit**
aprobadas. Los cinco fixtures ejecutables se compararon completos en el
motor y por HTTP real; los ocho de 4B siguen pendientes.
También pasaron go test ./..., go vet ./... y el verificador Python de la
Etapa 1 (30 JSON, 13 pares, nueve mutaciones inválidas de perfiles).

Se ejecutaron ambos procesos reales: Go aceptó secuencialmente los cinco
eventos de race-final, y GET /results se consultó dos veces en los cuatro
prefijos race-start, race-live-2, race-live-3 y race-final. Las ocho respuestas
completas coincidieron con sus expected aprobados. Ambos procesos de prueba
se detuvieron al terminar; no hay servicios que el usuario deba limpiar.
