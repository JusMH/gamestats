# Integrar un videojuego externo

GameStats analiza hechos; no ejecuta la lógica del juego ni su IA. El juego
mantiene las reglas y puntuación necesarias para jugar, y envía copias de sus
acontecimientos. La integración de un juego con IA en Prolog es **FUTURA: no
existe código Prolog ni un adaptador implementado**. Ese juego podría producir
el mismo HTTP/JSON, directamente o mediante un adaptador externo.

## Publicar acontecimientos

El productor usa el [sobre JSON v1](events.md). Cada partida se identifica por
(gameId, matchId), cada eventId es único dentro de ella y data es un objeto
flexible. Enviar cada objeto siguiente por separado a POST /events con
Content-Type: application/json, esperando HTTP 200/status accepted antes del
siguiente. El array presenta la secuencia; no se envía como un lote.

```json
[
  {
    "schemaVersion": 1,
    "eventId": "start",
    "timestamp": "2026-10-18T12:00:00Z",
    "gameId": "external-score",
    "matchId": "demo-01",
    "type": "MATCH_STARTED",
    "data": {"players": ["p1", "p2"]}
  },
  {
    "schemaVersion": 1,
    "eventId": "points-1",
    "timestamp": "2026-10-18T12:00:02Z",
    "gameId": "external-score",
    "matchId": "demo-01",
    "playerId": "p1",
    "type": "POINTS_EARNED",
    "data": {"points": 7}
  },
  {
    "schemaVersion": 1,
    "eventId": "finish",
    "timestamp": "2026-10-18T12:00:05Z",
    "gameId": "external-score",
    "matchId": "demo-01",
    "type": "MATCH_FINISHED",
    "data": {}
  }
]
```

Ejemplo para un objeto guardado en evento.json:

```sh
curl -X POST http://127.0.0.1:8080/events -H "Content-Type: application/json" --data-binary @evento.json
curl http://127.0.0.1:8080/results/external-score/demo-01
```

Consultar GET /results después del segundo evento produce estadísticas de la
partida open; tras el tercero corresponde al snapshot closed. Consultar no
cierra ni acumula. Sin nuevas solicitudes no hay push ni actualización enviada.
Un evento nuevo tras el cierre o un duplicado recibe 409; un acontecimiento
intermedio no crea una partida sin inicio.

## Añadir un perfil sin cambiar Go

Sin perfil, Go sigue aceptando el gameId; Scala valida el snapshot y responde
200 con metrics:[] y patterns:[], sin inventar puntuaciones. Para obtener
score.player, guardar este perfil como external-score.json en un directorio
de desarrollo, sin editar los tres perfiles aprobados:

```json
{
  "profileSchemaVersion": 1,
  "profileVersion": "1.0.0",
  "gameId": "external-score",
  "playersField": "/data/players",
  "capabilities": [
    {
      "templateId": "event-metrics/v1",
      "instanceId": "scores",
      "availability": "live",
      "metrics": [
        {
          "id": "score.player",
          "label": "Puntos por jugador",
          "eventType": "POINTS_EARNED",
          "operation": "sum",
          "scope": "player",
          "field": "/data/points",
          "numberKind": "integer",
          "minimum": 0,
          "unit": "points"
        }
      ]
    }
  ],
  "specializedAnalyzers": []
}
```

Con estos hechos, p1 suma 7 y p2 suma 0; las dos filas son provisional con
open y final con closed. El productor envía incrementos, no el acumulado
repetido. Un points ausente, texto o negativo invalida la salida afectada;
no se convierte en cero. Son reglas analíticas de Scala, no exigencias nuevas de Go.

count, sum, avg, duración y ventanas compatibles solo necesitan configuración.
Pueden coexistir varias instancias con IDs únicos: ver
[plantillas](analysis-templates.md). Otra reconstrucción de tablero o una regla
no expresable requiere un analizador Scala independiente, registro por analyzerId,
pruebas y compilación. No añadir condicionales por gameId al núcleo ni scripts,
expresiones arbitrarias o clases ejecutables a JSON.

## Validación y puesta en servicio controlada

1. Crear un directorio ignorado, por ejemplo docker/evidence/new-game/profiles,
   copiar allí los perfiles aprobados sin modificarlos y añadir external-score.json.
2. Con JDK 21 y las herramientas del [README Scala](../scala-analysis/README.md),
   iniciar un proceso de desarrollo con ese directorio. En PowerShell, desde la
   raíz (asignaciones solo para esa terminal):

```powershell
$env:SCALA_PROFILES_DIR = (Resolve-Path docker/evidence/new-game/profiles).Path
$env:SCALA_HOST = "127.0.0.1"
$env:SCALA_PORT = "9000"
Set-Location scala-analysis
.\sbtw.ps1 run
```

3. El arranque valida todos los perfiles: falla ante IDs/versiones desconocidos,
   duplicados, campos incompatibles o directorio ausente/vacío. /health acredita
   arranque, no exactitud de métricas.
4. Probar POST /analyze con un snapshot de los dos primeros eventos (state open)
   y otro con los tres (state closed). Comprobar sujetos, 7/0 puntos, estados,
   source.profileVersion y basis; repetir y enviar un dato inválido. Para el
   camino completo, iniciar Go con SCALA_SERVICE_URL=http://127.0.0.1:9000 y
   enviar los eventos por separado.
5. En una futura tarea de implementación del juego, añadir pruebas de motor/HTTP
   y escenarios propios. Ejecutar las suites existentes y el verificador de
   fixtures; no alterar oráculos aprobados para encubrir incompatibilidades.

Los perfiles se cargan una sola vez. Cambiarlos requiere reiniciar Scala;
editar un archivo del anfitrión no incorpora perfiles a una imagen publicada.
La entrega actual incluye los tres aprobados: extender esa imagen requiere
construcción/versionado revisados, fuera de esta auditoría. No hay registro
por API. Respetar [límites](architecture.md#límites-comprobados-en-código).

Mantener la puntuación crítica y las decisiones de IA dentro del videojuego:
GameStats es análisis bajo demanda con red y límites, no un reloj de juego ni
un servicio con persistencia o latencia garantizadas.
