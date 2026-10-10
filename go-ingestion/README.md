# Ingestión HTTP — Go (implementación 1)

Este módulo ejecuta la primera etapa de [la arquitectura](../docs/architecture.md):
recibe un evento y valida el sobre común del [contrato v1](../docs/events.md).
La respuesta confirma únicamente la validación.

## Preparación

Instalar Go 1.22 o posterior y comprobar `go version`. Desde la raíz del repositorio:

```sh
cd go-ingestion
go test ./...
```

El módulo ya incluye `go.mod` y usa exclusivamente la biblioteca estándar.
No hace falta ejecutar `go mod init`, instalar dependencias externas ni preparar bases de datos.

## Estructura

```text
go-ingestion/
  go.mod
  cmd/server/main.go                 # Inicio y configuración HTTP
  internal/events/event.go           # Sobre genérico y validación
  internal/events/event_test.go      # Conservación de data
  internal/httpapi/handler.go        # Endpoints y respuestas
  internal/httpapi/handler_test.go   # Pruebas del contrato por HTTP
```

## Iniciar el servidor

Desde `go-ingestion/`:

```sh
go run ./cmd/server
```

Escucha en `:8080` (puerto **8080**, todas las interfaces). Para detenerlo, usar Ctrl+C.
Si el puerto está ocupado, el servidor termina y muestra el error.
No se configura un motor de concurrencia ni estado de partidas en esta etapa;
la atención HTTP usa el servidor estándar de Go.

## Consultar la salud

```sh
curl http://localhost:8080/health
```

Respuesta HTTP 200:

```json
{"status":"ok"}
```

En Postman: método **GET**, URL `http://localhost:8080/health`.

## Enviar eventos

En Postman: método **POST**, URL `http://localhost:8080/events`,
Body → raw → JSON, y cabecera `Content-Type: application/json`.
Ejemplo de combate:

```json
{
  "schemaVersion": 1,
  "eventId": "evt-001",
  "timestamp": "2026-10-10T18:00:00Z",
  "gameId": "arena-combat",
  "gameVersion": "1.0",
  "matchId": "match-001",
  "playerId": "player-01",
  "type": "DAMAGE_DEALT",
  "data": {
    "targetId": "player-02",
    "amount": 35
  }
}
```

Guardar ese JSON como `event.json` y enviarlo desde la misma carpeta:

```sh
curl -X POST http://localhost:8080/events -H "Content-Type: application/json" --data-binary "@event.json"
```

En Windows PowerShell, usar `curl.exe` si `curl` es un alias de otro comando.

Respuesta HTTP 200:

```json
{"status":"validated","eventId":"evt-001"}
```

Ejemplo de carreras, que utiliza el mismo endpoint y código Go:

```json
{
  "schemaVersion": 1,
  "eventId": "race-001",
  "timestamp": "2026-10-10T12:00:00-06:00",
  "gameId": "racing",
  "matchId": "race-01",
  "playerId": "driver-01",
  "type": "LAP_COMPLETED",
  "data": {"lap": 2, "durationMs": 75000}
}
```

Un evento de partida puede omitir `playerId` y usar `data: {}`.

## Validación y errores

- El cuerpo debe contener exactamente un objeto JSON; se permite espacio en blanco al final.
- `schemaVersion` es obligatorio y debe escribirse como el entero `1`;
  se rechazan `"1"`, `1.0`, `null` y otras versiones.
- `eventId`, `gameId`, `matchId` y `type` son cadenas obligatorias no vacías.
  Una cadena formada únicamente por espacios también se rechaza.
- `timestamp` es una cadena obligatoria con fecha RFC3339 válida y zona horaria:
  `Z` o un desplazamiento como `-06:00`. Se admiten fracciones de segundo.
- `gameVersion` y `playerId` pueden omitirse; si están presentes, deben ser
  cadenas (pueden estar vacías). `null` no es una cadena.
- `data` debe estar presente y ser un objeto JSON; puede estar vacío.
  Go no exige ni valida atributos de combate, carreras u otros juegos.
- Los nombres del sobre son sensibles a mayúsculas y se rechazan claves
  repetidas en ese sobre. Los campos adicionales del sobre se permiten.
  `UPPER_SNAKE_CASE` es la convención documentada de `type`, no una lista
  cerrada ni una restricción adicional de esta validación.
- Cada solicitud admite hasta **1 MiB** de cuerpo. Un cuerpo que excede
  este límite devuelve HTTP 400. El servidor también configura tiempos
  máximos de lectura y escritura.

`data` se representa con `json.RawMessage`: conserva el JSON original y
permite posponer su interpretación sin estructuras por videojuego ni pérdida
de precisión por convertir números a `float64`. La sintaxis completa ya la
comprueba el decodificador; después solo se verifica que el valor sea un objeto.
Véase la [documentación oficial de RawMessage](https://pkg.go.dev/encoding/json#RawMessage).

JSON mal formado o un campo inválido devuelve HTTP 400 con un error claro, por ejemplo:

```json
{"error":"eventId is required"}
```

Los métodos diferentes de GET en `/health` y POST en `/events` devuelven
HTTP 405, JSON de error y la cabecera `Allow` con el método admitido.
Como establece HTTP, las respuestas a HEAD no transmiten un cuerpo por la red.
Las rutas desconocidas reciben el 404 estándar de Go.

## Pruebas y comprobaciones

Desde `go-ingestion/`:

```sh
go test ./...
go vet ./...
```

Para ver los casos individuales:

```sh
go test -v ./...
```

Las pruebas utilizan `testing` y `net/http/httptest`. Cubren combate, carreras,
partidas sin jugador, datos de juegos desconocidos, conservación de números
grandes, campos ausentes, valores vacíos o null, tipos incorrectos, timestamps
inválidos, versiones incorrectas, JSON mal formado, data que no es un objeto,
cuerpos múltiples, claves repetidas, el límite de tamaño, salud y métodos HTTP.

## Funciones pendientes

Esta implementación no almacena eventos ni comprueba IDs duplicados entre
solicitudes. No agrupa partidas, no mantiene estado, no cierra lotes con
`MATCH_FINISHED`, no implementa goroutines/channels para coordinar partidas,
no envía eventos a Scala ni ofrece resultados o estadísticas.
Tampoco incluye bases de datos, Docker, CI/CD, autenticación ni despliegue.

Las siguientes etapas incorporarán estado y duplicados por
`(gameId, matchId)`, coordinación concurrente y envío de partidas completas
a Scala, conforme a la arquitectura documentada. La semántica de los datos
específicos y el análisis estadístico siguen siendo responsabilidad de Scala.
