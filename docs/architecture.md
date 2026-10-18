# Arquitectura genérica de GameStats

Descripción del sistema implementado; los informes de etapas anteriores se
conservan como historia en el [índice](README.md). Los contratos v1 no cambian.

## Flujo y responsabilidades

```text
Productor externo / simulador -- POST /events --> Go
Cliente -- GET /matches/{gameId}/{matchId} --> estado de ingestión
Cliente -- GET /results/{gameId}/{matchId} --> Go copia snapshot open/closed
                                             |
                                             +-- POST /analyze --> Scala
                                             <-- métricas/patrones
```

El productor detecta los hechos (por ejemplo, una vuelta terminada). Go valida
el sobre JSON, agrupa por (gameId, matchId), protege el estado en memoria y
controla inicio, cierre y duplicados. No interpreta damage, points o tableros.
Scala valida el snapshot y los datos que consumen sus reglas, carga un perfil,
ejecuta capacidades puras y compone resultados. No decide la aceptación en Go.

Cerrar una partida no dispara análisis. Cada GET /results recalcula todos los
eventos del snapshot; no hay resultados almacenados, push, SSE ni WebSockets.
Un snapshot consistente puede quedar desactualizado mientras viaja por HTTP.
Repetir los mismos eventos, orden y versión de perfil produce el mismo JSON
semántico. Los resultados provisionales pueden cambiar con nuevos eventos;
el cierre fija los eventos, no la versión futura del analizador.

## Tres niveles componibles

| Nivel | Implementación | Extensión |
| --- | --- | --- |
| Núcleo genérico | Go coordina; Scala valida y compone sin condicionales por gameId. | Nuevo juego no cambia Go. |
| Capacidades reutilizables | event-metrics/v1, duration/v1, temporal-window/v1, seleccionadas por perfil. | count/sum/avg nuevos requieren configuración compatible. |
| Especialización | Registro por analyzerId; tic-tac-toe-line/v1 reconstruye el tablero. | Nueva regla especializada requiere código Scala, registro, pruebas y compilación. |

Racing combina conteo/suma/promedio de vueltas con duración; Snake incorpora
ventanas; Gato incorpora tablero. Los perfiles son datos declarativos,
sin eval ni carga arbitraria de clases. Se cargan una vez al iniciar Scala;
no hay API de registro o recarga dinámica. Ver [plantillas](analysis-templates.md)
y [guía de extensión](integrating-new-games.md).

## Concurrencia real de Go

net/http atiende solicitudes mediante goroutines administradas por Go. No hay
una cola de eventos propia ni workers asíncronos en el servidor de ingestión.
Un RWMutex protege el mapa de partidas y cada partida tiene su propio RWMutex.
La creación publica la partida de forma atómica; las operaciones sobre una
existente liberan el bloqueo del registro antes de bloquear esa partida.

En una partida, comprobar duplicados, validar el ciclo de vida, agregar y cerrar
se realiza en la misma sección crítica. Distintas partidas pueden progresar
simultáneamente; el acceso breve al registro sigue siendo compartido. El orden
de aceptación depende de las escrituras sincronizadas, no del timestamp ni de
una garantía de orden de llegada entre clientes. El productor debe esperar
cada aceptación antes del siguiente evento de su partida.

GetEvents copia conjuntamente estado, eventos y bytes de data bajo bloqueo de
lectura; libera los bloqueos antes de serializar y llamar a Scala. Cada consulta
observa una copia consistente, sin mantener un mutex durante la red. Las pruebas
cubren inicios en competencia, escrituras/cierre, copias defensivas y avance de
otra partida cuando una está bloqueada. CI aprobó ambos detectores de carreras;
es evidencia de los casos ejecutados, no una prueba universal de corrección.

El simulador sí utiliza goroutines por partida y canales como barreras de fase.
Es un cliente independiente; sus mecanismos no implican colas en Go ingestion.
Scala usa HttpServer del JDK y un pool de cuatro workers con cola de 64 tareas y
CallerRunsPolicy: bajo saturación puede ejecutar también el hilo que presenta
la tarea. No es un límite absoluto de cuatro peticiones en ejecución.

## Motor funcional Scala

Los archivos reales están en scala-analysis/src/main/scala/gamestats:

| Archivo | Responsabilidad |
| --- | --- |
| Model.scala, Json.scala | Modelos inmutables y JSON decimal. |
| Decoding.scala | Validación tipada de snapshots y perfiles; carga inicial. |
| Engine.scala | Registros, composición, count/sum/avg y duración. |
| TemporalWindow.scala | Ventanas inclusivas greedy sin solapamiento. |
| TicTacToeLine.scala | Fold del tablero y primera línea ganadora. |
| HttpService.scala, Main.scala | Transporte HTTP, límites y configuración. |

filter selecciona candidatos; groupBy separa sujetos; map transforma valores;
fold acumula decimales y reconstruye tableros; flatMap compone salidas. collect
extrae casos tipados donde corresponde, sin convertir errores numéricos en cero.
Los errores esperados se representan mediante valores. La suma utiliza
BigDecimal; el promedio se redondea HALF_EVEN a seis decimales al preparar la
salida. No hay contadores persistentes entre solicitudes ni estadísticas globales
entre partidas. «Multijuego» permite varios perfiles en un servicio, con una
respuesta por snapshot de una sola partida.

## Límites comprobados en código

MiB equivale a 1 048 576 bytes. Estos límites son validaciones, no benchmarks.

| Componente | Límite y efecto |
| --- | --- |
| POST /events Go | Cuerpo de 1 MiB; exceso devuelve 400. No valida campos particulares del juego. |
| Memoria Go | Sin límite global de partidas/eventos, expiración o paginación; incluye cerradas hasta detener Go. |
| Cliente Go → Scala | Timeout total 5 s, sin retries ni redirecciones; respuesta máxima 8 MiB. No fija un límite al cuerpo saliente antes de construir el snapshot. |
| HTTP Scala | Petición/respuesta de 8 MiB; exceso de petición 413, respuesta demasiado grande 503. Go convierte HTTP upstream distinto de 200 en 502. |
| Snapshot Scala | Entre 1 y 10 000 eventos; no se particiona automáticamente. |
| Jugadores con perfil conocido | Hasta 1 000 en roster/sujetos relevantes; no hay un catálogo de jugadores en Go. |
| Perfiles Scala | Hasta 128 archivos, 1 MiB por archivo; carga al inicio, directorio ausente/vacío o perfil inválido impide arrancar. |
| Composición por perfil | Hasta 128 instancias de capacidades/analizadores y 1 000 IDs de salida únicos; event-metrics admite 1–256 métricas por instancia. |
| Resultados Scala | Guarda preventiva estimada de 20 000 filas, basada en IDs de salida y jugadores; puede ser conservadora. |
| JSON Scala | Profundidad 64, cadenas 65 536 caracteres, token numérico 128 caracteres; precisión decimal 128 y escala absoluta hasta 1 000. Rechaza claves repetidas, también dentro de data. |
| Simulador | Concurrencia configurable 2–16; timeout por solicitud y duración total; respuestas de hasta 8 MiB. |

Go puede aceptar eventos cuyo snapshot acumulado exceda después los límites de
Scala. Aceptación de ingestión no garantiza que una partida de cualquier tamaño
sea analizable. Copias completas y polling tienen coste creciente; no se promete
rendimiento para cientos de miles de eventos sin mediciones.

## Construcción, publicación y deployment

Ambos módulos Go declaran 1.22; CI y las imágenes prueban 1.26.5. No se ha
demostrado incompatibilidad con 1.22 ni se presenta como versión ejecutada.
Scala 3.3.7 y sbt 1.11.7 usan JDK 21; CI selecciona Temurin 21 disponible,
mientras las imágenes fijan la actualización y digest indicados en [Docker](docker.md).

Dockerfiles multietapa empaquetan servicios independientes y un simulador finito.
Compose utiliza DNS interno, healthchecks y usuarios sin privilegios; Go publica
127.0.0.1:8080 y Scala no publica su puerto al anfitrión. El deployment elegido
es local, Linux/amd64 con Docker Desktop o motor compatible. GHCR distribuye
imágenes; GitHub Release distribuye el ZIP por digest; ninguno aloja una API
remota. Ver [distribución](academic-distribution.md) para la evidencia y el fallo
conocido de Finalize, y [matriz académica](academic-audit.md).

## Garantías que no ofrece el sistema

No hay persistencia, recuperación tras reinicio, estado compartido entre réplicas,
exactamente una vez, reintentos idempotentes, reapertura, caché o procesamiento
incremental. Sin perfil, Scala devuelve arrays vacíos tras validar la estructura;
no inventa estadísticas. Gato comprueba la evidencia definida en [games.md](games.md),
sin certificar toda la legalidad. No se implementan física de Snake, IA, Prolog,
reglas universales, nube ni rendimiento distribuido. Los contratos completos
están en [el índice](README.md).
