# Ingestión y gestión de partidas — Go (implementación 3)

El servicio valida el sobre del [contrato v1](../docs/events.md) y conserva
temporalmente los eventos por `(gameId, matchId)`, conforme a
[la arquitectura](../docs/architecture.md). Go coordina la ingestión y el
estado; las estadísticas y los patrones corresponden a Scala.

## Preparación y ejecución

Instalar Go **1.22 o posterior** y verificar `go version`.
Desde la raíz del repositorio:

```sh
cd go-ingestion
go test ./...
go run ./cmd/server
```

El servicio escucha en `:8080` (**puerto 8080**, todas las interfaces).
Ctrl+C lo detiene; al reiniciar, se pierde todo el estado.
Si el puerto está ocupado, el servidor muestra el error y termina.
El módulo incluye `go.mod` y solo usa la biblioteca estándar:
no hay dependencias externas ni pasos de inicialización.

## Organización

```text
cmd/server/main.go                  Inicio y configuración HTTP
internal/events/event.go            Validación genérica del sobre v1
internal/events/event_test.go        Conservación del JSON data
internal/matches/manager.go          Registro, eventos y ciclo de vida
internal/matches/manager_test.go     Estado, propiedad y concurrencia
internal/httpapi/handler.go          Ingestión, salud y consulta
internal/httpapi/handler_test.go     Validación y métodos HTTP
internal/httpapi/matches_test.go     Ciclo de vida y HTTP concurrente
```

## Ciclo de vida y política de duplicados

1. `MATCH_STARTED` crea una partida **open** y se almacena como primer evento.
2. Los demás eventos necesitan una partida iniciada y abierta.
3. `MATCH_FINISHED` se almacena y cambia el estado a **closed**.
4. Tras el cierre se rechazan nuevos eventos; no se permite reiniciar una
   partida ni reutilizar su combinación `(gameId, matchId)` en este proceso.

**MATCH_STARTED debe haber sido aceptado antes de enviar los demás eventos.**
Esta simplificación concreta el ciclo de vida de esta etapa sin modificar el
sobre v1. El productor también debe esperar la aceptación de todos los
eventos antes de enviar `MATCH_FINISHED`. Si cierre y nuevos eventos llegan
simultáneamente, el orden de adquisición del bloqueo decide cuáles se
aceptan antes del cierre; los restantes se rechazan.

Cada `eventId` debe ser único dentro de una partida. Repetirlo devuelve
**409 Conflict**, aunque el contenido sea idéntico o la partida esté cerrada.
La detección de duplicados tiene prioridad sobre el estado cerrado.
Los eventos rechazados no se almacenan ni alteran el estado.
La misma partida o el mismo eventId en juegos diferentes no produce conflicto;
el mismo eventId también puede usarse en partidas distintas del mismo juego.

Se conservan todos los eventos aceptados, incluidos inicio y cierre.
El orden de almacenamiento es el de aceptación, no el de timestamp.
Los timestamps no se utilizan para imponer el ciclo de vida.

Para admitir reintentos idempotentes posteriormente habría que comparar
el contenido con el evento ya aceptado y devolver la confirmación original
si coincide, manteniendo 409 si difiere. Haría falta definir igualdad del
JSON (incluido el orden de claves), conservar el resultado de aceptación
y precisar la retención y persistencia de esos registros. Esta versión
no implementa esa política.

## Arquitectura y seguridad concurrente

Cada instancia de handler posee un administrador independiente.
El administrador mantiene un mapa cuya clave es una estructura con
`gameId` y `matchId`, evitando colisiones por concatenación de cadenas.

Un `sync.RWMutex` protege las búsquedas y creaciones del registro.
La creación publica la partida con el primer evento e índice de IDs ya
completos. Cada partida posee otro `sync.RWMutex`: sus escrituras comprueban
duplicados y ciclo de vida, agregan el evento y actualizan el estado dentro
de una única sección crítica. Las consultas usan un bloqueo de lectura
y copian los IDs para devolver un estado consistente.

El bloqueo del registro se libera antes de bloquear una partida existente.
Así las partidas diferentes avanzan independientemente y las operaciones
de una misma partida son atómicas. Los bytes de `data` se copian al guardar
para evitar que el solicitante pueda modificar el evento almacenado.
El administrador recibe eventos ya validados por `internal/events`.

`net/http` ya atiende conexiones concurrentes; no se añaden goroutines
por evento, workers ni channels de producción innecesarios.
Los mutexes son suficientes para este almacenamiento síncrono en memoria.
Las pruebas utilizan goroutines y barreras para provocar competencia real,
incluyendo solicitudes a un servidor `httptest.NewServer`.

## Endpoints y códigos HTTP

| Endpoint / resultado | Código | Respuesta |
| --- | --- | --- |
| GET /health | 200 | `{"status":"ok"}` |
| POST /events, almacenado en memoria | 200 | `{"status":"accepted","eventId":"..."}` |
| POST /events, JSON o sobre inválido | 400 | `{"status":"rejected","error":"..."}` |
| POST /events, partida no iniciada | 404 | Rechazo con error explicativo |
| POST /events, duplicado, segundo inicio o evento tras cierre | 409 | Rechazo con error explicativo |
| GET /matches/{gameId}/{matchId}, existente | 200 | Resumen de ingestión |
| GET /matches/{gameId}/{matchId}, inexistente | 404 | `{"error":"..."}` |
| Método no permitido en estas rutas | 405 | JSON de error y cabecera `Allow` |
| Ruta desconocida o incompleta | 404 | Respuesta estándar de Go |

La aceptación confirma el almacenamiento temporal en este proceso;
no implica persistencia durable, procesamiento estadístico ni análisis.
El antiguo estado `validated` se sustituye por `accepted`.

Las respuestas usan `Content-Type: application/json`, salvo el 404 estándar
de rutas desconocidas. HEAD recibe 405 y Allow; por las reglas de HTTP,
el servidor no transmite su cuerpo por la red.
Codificar cada identificador como un segmento de URL cuando contenga
caracteres especiales, por ejemplo `new%2Fgame`.

## Ejemplo completo con Postman

Usar **POST http://localhost:8080/events**, Body → raw → JSON y cabecera
`Content-Type: application/json`. Enviar estos cuerpos **en orden**,
esperando HTTP 200 para cada uno.

Inicio:

```json
{
  "schemaVersion": 1,
  "eventId": "start-001",
  "timestamp": "2026-10-10T18:00:00Z",
  "gameId": "arena-combat",
  "matchId": "match-001",
  "type": "MATCH_STARTED",
  "data": {}
}
```

Evento:

```json
{
  "schemaVersion": 1,
  "eventId": "evt-001",
  "timestamp": "2026-10-10T18:00:01Z",
  "gameId": "arena-combat",
  "gameVersion": "1.0",
  "matchId": "match-001",
  "playerId": "player-01",
  "type": "DAMAGE_DEALT",
  "data": {"targetId": "player-02", "amount": 35}
}
```

Cierre:

```json
{
  "schemaVersion": 1,
  "eventId": "finish-001",
  "timestamp": "2026-10-10T18:00:02Z",
  "gameId": "arena-combat",
  "matchId": "match-001",
  "type": "MATCH_FINISHED",
  "data": {}
}
```

Consultar con **GET http://localhost:8080/matches/arena-combat/match-001**:

```json
{
  "gameId": "arena-combat",
  "matchId": "match-001",
  "state": "closed",
  "eventCount": 3,
  "eventIds": ["start-001", "evt-001", "finish-001"]
}
```

Antes del cierre, el estado es `open`. Reenviar `evt-001` devuelve 409:

```json
{"status":"rejected","error":"eventId already exists in this match"}
```

Enviar un nuevo eventId tras el cierre también devuelve 409.
Enviar un evento de una partida no iniciada o consultar una inexistente
devuelve 404.

Para probar carreras, crear primero otra partida con `gameId: "racing"`
y `type: "MATCH_STARTED"`. Después enviar:

```json
{
  "schemaVersion": 1,
  "eventId": "race-001",
  "timestamp": "2026-10-10T12:00:01-06:00",
  "gameId": "racing",
  "matchId": "match-001",
  "playerId": "driver-01",
  "type": "LAP_COMPLETED",
  "data": {"lap": 2, "durationMs": 75000}
}
```

Esta partida permanece separada de `arena-combat/match-001`.
Un juego desconocido usa exactamente el mismo flujo y admite data arbitrario.

## Ejemplos con curl

```sh
curl http://localhost:8080/health
curl http://localhost:8080/matches/arena-combat/match-001
```

Guardar cada cuerpo de Postman en un archivo y enviarlos en orden:

```sh
curl -X POST http://localhost:8080/events -H "Content-Type: application/json" --data-binary "@start.json"
curl -X POST http://localhost:8080/events -H "Content-Type: application/json" --data-binary "@event.json"
curl -X POST http://localhost:8080/events -H "Content-Type: application/json" --data-binary "@finish.json"
```

En Windows PowerShell, usar `curl.exe` si `curl` es un alias de otro comando.

## Validación del contrato v1

La validación previa al almacenamiento se conserva:

- Exactamente un objeto JSON en el cuerpo; sin contenido adicional.
- `schemaVersion`: entero obligatorio `1`.
- `eventId`, `gameId`, `matchId` y `type`: cadenas obligatorias no vacías
  ni formadas solamente por espacios.
- `timestamp`: fecha válida RFC3339 con zona horaria; admite fracciones.
- `gameVersion` y `playerId`: cadenas opcionales; si están presentes,
  no pueden ser null, pero pueden estar vacías.
- `data`: objeto obligatorio, posiblemente vacío. Se conserva como
  `json.RawMessage`, sin convertir números a float64 ni interpretar su
  significado por videojuego.
- Nombres de campos sensibles a mayúsculas, sin claves repetidas en el sobre;
  se permiten claves adicionales.
- `UPPER_SNAKE_CASE` es una convención para type, no una lista cerrada.
  Solo `MATCH_STARTED` y `MATCH_FINISHED` controlan el ciclo de vida.
- Límite de **1 MiB** por solicitud (excederlo devuelve 400), con tiempos
  máximos HTTP de lectura y escritura.

Go no exige atributos como damage, weapon, lap ni duration.

## Pruebas

Desde `go-ingestion/`:

```sh
go test ./...
go vet ./...
go test -race ./...
```

El detector de carreras requiere cgo habilitado y un compilador C compatible.
Si no están disponibles, el tercer comando no puede ejecutarse y no sustituye
las pruebas normales. Para ver casos individuales, usar `go test -v ./...`.

Se conservan los casos de validación de la implementación 1, preparando
una partida para los eventos válidos que no son MATCH_STARTED.
Las pruebas nuevas cubren creación, cierre, duplicados, errores sin
mutación, claves independientes, data arbitrario, copias defensivas,
inicios en competencia, varias partidas concurrentes por HTTP, escrituras
simultáneas sobre una partida, consultas consistentes y cierre concurrente.
Una prueba bloquea una partida y confirma que otra puede seguir avanzando.

## Limitaciones y siguientes etapas

Los eventos y las partidas permanecen en memoria hasta detener el proceso,
incluidas las partidas cerradas. No hay expiración, eliminación, paginación
ni límites globales de partidas/eventos: el uso de memoria puede crecer
y consultar todos los IDs cuesta tiempo y espacio proporcional a la partida.

El estado es local a una instancia; varios procesos no comparten partidas.
No hay persistencia durable, reintentos idempotentes ni recuperación tras fallos.
GET /matches muestra solo estado de ingestión; GET /results delega las estadísticas al servicio Scala, si está disponible.

Se envían snapshots a Scala bajo demanda; los resultados analíticos no se almacenan.
No hay bases de datos, CI/CD, interfaz gráfica ni despliegue; Docker se documenta al final.
El servicio Scala real ya está integrado y verificado. Quedan pendientes
las políticas de reintentos y retención.

## Integración con Scala bajo demanda

Se implementa exclusivamente el mecanismo HTTP; **utiliza el servicio
real desarrollado en Scala**. El [contrato de integración](../docs/integration-api.md)
conserva el transporte existente y ahora define los elementos analíticos para implementar Scala; documenta los límites
y la política exacta de errores.

```text
GET /results/{gameId}/{matchId}
  → Manager.GetEvents: copia consistente y profunda de los eventos
  → cliente analysis: POST {SCALA_SERVICE_URL}/analyze
  → Scala: métricas y patrones
  → Go: valida la respuesta y la devuelve
```

`internal/matches` no depende de HTTP. `internal/analysis` implementa el cliente
reutilizable; `httpapi.Analyzer` permite inyectarlo en el handler. El constructor
anterior `NewHandler()` sigue funcionando, con integración desconfigurada.
`cmd/server` lee la variable de entorno e inyecta el cliente.

Configurar antes de arrancar (URL base, sin `/analyze`):

```powershell
$env:SCALA_SERVICE_URL = 'http://localhost:9000'
cd go-ingestion
go run ./cmd/server
```

En una shell POSIX: `SCALA_SERVICE_URL=http://localhost:9000 go run ./cmd/server`.
Sin variable, los endpoints existentes funcionan y results responde 503 para
una partida existente. Una URL inválida impide iniciar el servidor.

Enviar primero los eventos de los ejemplos anteriores. Consultar antes o
después de MATCH_FINISHED:

```sh
curl http://localhost:8080/results/arena-combat/match-001
```

En Postman usar GET con la misma URL, sin cuerpo. Ejemplo de estructura de
respuesta (los valores reales los define Scala):

```json
{"gameId":"arena-combat","matchId":"match-001","state":"open","metrics":[],"patterns":[]}
```

| Resultado de GET /results | Código |
| --- | --- |
| Respuesta válida de Scala | 200 |
| Partida inexistente (prioridad sobre configuración) | 404 |
| Método distinto de GET | 405, Allow: GET |
| Scala sin configurar | 503 |
| Conexión fallida, HTTP de Scala distinto de 200, redirección o respuesta inválida | 502 |
| Timeout o deadline | 504 |

El timeout total es 5 segundos. Se propaga la cancelación del consumidor;
no se siguen redirecciones, ni se reintenta, ni se exponen errores internos.
Los resultados exigen identificadores/estado coincidentes y metrics/patterns
como arreglos no null. El límite de respuesta es 8 MiB.

Cada GET vuelve a analizar todos los eventos aceptados hasta su snapshot;
no hay caché, push ni análisis automático al cerrar. Durante la llamada HTTP
pueden seguir llegando eventos: la respuesta corresponde al snapshot enviado,
no necesariamente al estado de la partida al recibir la respuesta.
El resumen de `/matches` conserva su formato y no llama a Scala.

### Prueba sin Scala real

Los servidores simulados viven en las pruebas y no implementan análisis:

```sh
cd go-ingestion
go test -v ./internal/analysis ./internal/httpapi -run 'TestClient|TestResults'
```

`TestClientRequestAndResult` simula POST /analyze con `httptest.Server` y
verifica el JSON completo. `TestResultsLifecycle` ingiere eventos, consulta
partidas abiertas y cerradas y devuelve datos de prueba del servidor simulado.
`TestResultsFailures` simula indisponibilidad, timeout, HTTP de error y JSON
inválido. `TestResultsConcurrentIngestion` mantiene ocho análisis en vuelo
mientras ingresan eventos y se cierra la partida, comprobando que la red no
retiene el mutex. Estos servidores existen solo mientras se ejecuta el test;
no son un servicio Scala ni devuelven métricas reales.

Las nuevas pruebas de snapshots verifican contenido, copia defensiva y
lecturas/escrituras concurrentes. Se conservan todas las pruebas anteriores.
Para probar manualmente con un mock externo, configurarlo para responder a
POST /analyze con HTTP 200 y el JSON de respuesta anterior, haciendo coincidir
identificadores y state; iniciar Go con su URL base y usar los ejemplos de
POST /events y GET /results. Para verificar el análisis completo, utilizar
el servicio Scala real según su README o la guía Docker.

## Diseño analítico de Etapa 1

El transporte Go existente no cambia. Los [perfiles componibles](../docs/analysis-templates.md) y los [fixtures](../docs/fixtures/analysis-v1/README.md) definen el contrato analítico de Gato, Snake y racing, implementado en Scala 4A/4B; véase el README de Scala. Ver [resultados provisionales y finales](../docs/metrics-and-patterns.md). Los ejemplos de combate anteriores siguen siendo ejemplos genéricos de ingestión, no analizadores del MVP.

## Ejecución con Docker

Desde la raíz del repositorio:

~~~sh
docker compose up -d --build
~~~

La imagen Go es multietapa (Go 1.26.5 → Alpine 3.23.4, bases fijadas por digest),
con usuario 10001:10001, healthcheck real y sin fuentes/compilador en runtime.
Compose configura SCALA_SERVICE_URL=http://scala-analysis:9000 y publica
127.0.0.1:8080. No se modifican los endpoints ni la implementación.
El proceso conserva todas las partidas en memoria: detenerlo/recrearlo las pierde.
[Guía Docker y procedimiento de integración](../docs/docker.md).
