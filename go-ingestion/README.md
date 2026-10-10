# Ingestión y gestión de partidas — Go (implementación 2)

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
Las consultas muestran solo estado de ingestión; no incluyen estadísticas.

Todavía no se envían lotes a Scala ni se almacenan resultados analíticos.
No se implementan bases de datos, Docker, CI/CD, interfaz gráfica ni despliegue.
Las próximas etapas definirán la entrega de partidas cerradas a Scala,
el manejo de fallos/reintentos y las políticas de retención.
