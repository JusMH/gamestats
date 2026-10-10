# Contrato de eventos — GameStats (propuesta v1)

> Estado: diseño inicial para revisión del equipo. El formato es un acuerdo **entre componentes**, no una estructura propia de Go. Se basa en el enunciado del proyecto y en el enfoque híbrido acordado: núcleo genérico + métricas declarativas sencillas + analizadores especializados en Scala.

## Principio de diseño

GameStats debe poder **recibir** eventos de un juego desconocido sin agregar nuevos tipos de estructuras en Go. Eso **no** significa que pueda inferir automáticamente el significado de cualquier dato: el análisis requiere una configuración de métricas o un analizador que conozca su semántica.

- **Sobre (envelope) común**: identificación, tiempo y relación con juego y partida.
- **`data` flexible**: objeto JSON arbitrario definido por el productor de eventos.
- **Go**: valida el sobre, acepta y agrupa eventos, coordina procesamiento concurrente.
- **Scala**: valida semántica cuando es necesaria, aplica métricas configuradas y ejecuta reglas de secuencia.

## Formato de un evento

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

### Campos comunes

| Campo | Obligatorio | Tipo | Regla v1 |
| --- | --- | --- | --- |
| `schemaVersion` | Sí | entero | `1`, versión del sobre de GameStats, no del juego |
| `eventId` | Sí | cadena no vacía | identifica un evento; no puede repetirse dentro de la misma partida |
| `timestamp` | Sí | cadena | fecha/hora RFC 3339 con zona horaria, p. ej. `2026-10-10T18:00:00Z` |
| `gameId` | Sí | cadena no vacía | juego de origen |
| `gameVersion` | No | cadena | versión del videojuego, independiente de `schemaVersion` |
| `matchId` | Sí | cadena no vacía | partida dentro de un juego |
| `playerId` | No | cadena | omitir para eventos de toda la partida, como `MATCH_STARTED` |
| `type` | Sí | cadena no vacía | convención `UPPER_SNAKE_CASE` |
| `data` | Sí | objeto JSON | puede ser `{}`; significado según el tipo de evento y el juego |

`action` es un campo ilustrativo del enunciado, **no requerido** en nuestra v1: preferimos identificar la acción con `type` y poner los atributos variables en `data`. Podría incorporarse después si surge una necesidad concreta.

La clave de agrupación es **(`gameId`, `matchId`)**, no solo `matchId`. En v1 rechazaremos identificadores de eventos duplicados dentro de esa clave; si ya existe el ID con otro contenido, también se considera conflicto. Go debe verificar que `data` sea un objeto, sin exigir campos como `amount`, `lap` o `weapon`.

## Tipos iniciales para la demostración

Eventos comunes:
- `MATCH_STARTED`: inicio de partida (`playerId` omitido, `data: {}`).
- `MATCH_FINISHED`: final de partida (`playerId` omitido, `data: {}`).

Combate (`gameId: "arena-combat"`):
- `DAMAGE_DEALT`: requiere `playerId`; `data.targetId` (cadena) y `data.amount` (número no negativo).
- `PLAYER_ELIMINATED`: requiere `playerId` (atacante); `data.targetId` (jugador eliminado).

Carreras (`gameId: "racing"`):
- `LAP_COMPLETED`: requiere `playerId`; `data.lap` (entero positivo) y `data.durationMs` (entero positivo).
- `POSITION_CHANGED`: requiere `playerId`; `data.position` (entero positivo).

Estos campos específicos los valida la lógica de análisis de Scala para cada juego. El simulador producirá ejemplos válidos e inválidos, varias partidas y jugadores, y juegos en paralelo.

## Métricas híbridas

### Nivel A: métricas declarativas sencillas

Un archivo de configuración describe operaciones reutilizables. Propuesta de sintaxis (se formalizará junto con el motor Scala):

```json
{
  "gameId": "arena-combat",
  "metrics": [
    {
      "id": "damage_total",
      "label": "Daño total",
      "scope": "player",
      "eventType": "DAMAGE_DEALT",
      "operation": "sum",
      "field": "data.amount",
      "unit": "HP"
    },
    {
      "id": "eliminations",
      "label": "Eliminaciones",
      "scope": "player",
      "eventType": "PLAYER_ELIMINATED",
      "operation": "count",
      "unit": "kills"
    }
  ]
}
```

En v1 proponemos únicamente `count`, `sum` y `avg`, con `scope` igual a `player` o `match`. Para `sum` y `avg`, `field` deberá señalar a un dato numérico del evento. En `player`, los eventos sin `playerId` no aportan valores. Las definiciones se cargan en **Scala**, no dentro de Go.

Un juego desconocido y sin configuración todavía podrá ingresar eventos. GameStats podrá ofrecer al menos conteos genéricos por tipo, pero **no** inventará estadísticas de dominio.

### Nivel B: analizadores especializados en Scala

Para reglas que combinen eventos y exijan interpretar su secuencia, implementaremos analizadores asociados a un `gameId`. Primera regla propuesta:

**Eliminación tras daño**: el mismo atacante inflige `DAMAGE_DEALT` a un objetivo y luego lo elimina mediante `PLAYER_ELIMINATED` dentro de los siguientes 10 segundos de tiempo de evento. Debe correlacionar `playerId`, `data.targetId` y `timestamp`; no basta con contar eventos separados.

Como otra estadística combinada, Scala podrá calcular, por jugador, la razón entre eliminaciones y daño infligido cuando el caso de uso lo justifique. Definiremos explícitamente cómo manejar divisiones por cero.

## Secuencia y finalización

- Go agrupa acontecimientos de una partida hasta `MATCH_FINISHED` y envía una **colección completa** a Scala.
- Los timestamps pueden no coincidir con el orden de recepción: Scala deberá ordenar por `timestamp` antes de buscar patrones; si dos eventos tienen exactamente el mismo tiempo, necesitaremos una política de desempate determinista y no suponer causalidad.
- Para la primera versión, el simulador garantizará que `MATCH_FINISHED` sea enviado **después** de que Go haya aceptado los demás eventos de esa partida, incluso cuando existan partidas distintas simultáneas.
- Eventos nuevos después del cierre se rechazarán en v1. Soportar eventos tardíos, reintentos durables y reabrir partidas será una mejora posterior.

## Resultados propuestos

```json
{
  "gameId": "arena-combat",
  "matchId": "match-001",
  "status": "analyzed",
  "metrics": [
    {
      "id": "damage_total",
      "scope": "player",
      "subjectId": "player-01",
      "label": "Daño total",
      "value": 35,
      "unit": "HP"
    }
  ],
  "patterns": [
    {
      "id": "damage_then_elimination",
      "playerId": "player-01",
      "count": 1
    }
  ]
}
```

Esta salida es ilustrativa; los valores deberán corresponder a los eventos analizados realmente. Es una estructura genérica para que cualquier cliente consulte o presente métricas sin programar pantallas especiales para cada juego.

## Criterios de aceptación del contrato (primera implementación)

1. Go acepta un evento con `data` arbitrario si el sobre es válido.
2. Go rechaza un sobre sin `eventId`, sin `gameId` o con `timestamp` mal formado.
3. `MATCH_STARTED` es válido sin `playerId`.
4. Dos partidas con el mismo `matchId`, pero diferente `gameId`, se mantienen separadas.
5. Scala puede calcular `sum` y `count` configurados para combate.
6. Scala identifica correctamente una secuencia daño → eliminación y descarta una secuencia que exceda los 10 segundos.
7. El simulador prueba al menos dos juegos, múltiples partidas y eventos inválidos.

## Decisiones pendientes

- Definir si Go persistirá el estado entre reinicios; inicialmente será memoria temporal.
- Formalizar la respuesta HTTP para eventos inválidos y conflictos.
- Elegir la política final para tiempos iguales y llegadas tardías.
- Fijar esquema/versionado y mecanismo de carga de configuración del motor Scala.
- Confirmar con pruebas el contrato antes de declararlo estable.
