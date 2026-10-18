# Contrato común de eventos v1

Contrato vigente e implementado. Go valida el sobre y el ciclo de vida;
Scala implementa las capacidades y analizadores descritos en los documentos
enlazados. Esta revisión documental no modifica el contrato v1.

## Sobre estable

```json
{
  "schemaVersion": 1,
  "eventId": "apple-1",
  "timestamp": "2026-10-17T18:00:01Z",
  "gameId": "snake",
  "gameVersion": "1.0",
  "matchId": "match-001",
  "playerId": "p1",
  "type": "APPLE_EATEN",
  "data": {"points": 10}
}
```

| Campo | Regla de ingestión Go |
| --- | --- |
| schemaVersion | Obligatorio; entero JSON escrito como 1, no "1" ni 1.0. |
| eventId | Cadena no vacía ni solo espacios; única por (gameId, matchId). |
| timestamp | Cadena RFC3339 válida con zona Z o desplazamiento; admite fracciones. |
| gameId | Cadena no vacía ni solo espacios; no existe catálogo obligatorio en Go. |
| gameVersion | Cadena opcional; no es la versión del perfil analítico. |
| matchId | Cadena no vacía ni solo espacios. |
| playerId | Cadena opcional; puede omitirse para eventos de partida. |
| type | Cadena no vacía; UPPER_SNAKE_CASE es una convención, no una enumeración cerrada. |
| data | Objeto JSON obligatorio; admite {} y contenido desconocido. |

Los opcionales presentes deben ser cadenas: null se rechaza; "" se acepta
y Go omite esos opcionales vacíos al transmitir un snapshot.
Los nombres son sensibles a mayúsculas. Se rechazan claves repetidas del
sobre, cuerpos múltiples y JSON inválido. Límite de ingestión: 1 MiB.
Los identificadores no se recortan ni normalizan: "p1" y " p1" son distintos.
Para los fixtures y productores recomendamos identificadores sin espacios
periféricos. Los campos adicionales del sobre se aceptan, pero no se
conservan en el modelo Go. Las extensiones deben ir dentro de data.

data usa json.RawMessage: se conserva su contenido numérico y estructural
sin exigir atributos del videojuego ni convertir números a float64.
La serialización puede cambiar espacios y escapes; no garantiza igualdad
textual del JSON. No debe codificarse información significativa en claves
repetidas dentro de data: aunque Go puede aceptarlas, el parser JSON de Scala
las rechaza con HTTP 400; GET /results lo presenta como 502. No es una fila
analítica invalid ni una garantía de conservación de JSON ambiguo.

## Ciclo de vida ya implementado

- MATCH_STARTED debe ser el primer evento aceptado. Crea una partida open.
- Solo existe una partida por (gameId, matchId) durante la vida del proceso.
- Eventos siguientes se agregan en orden de aceptación.
- MATCH_FINISHED se almacena y cambia el estado a closed.
- No se admiten eventos nuevos después del cierre ni un segundo inicio.
- Duplicados devuelven 409 aunque sean idénticos o la partida ya esté cerrada.
- Eventos rechazados no cambian el estado ni el conjunto almacenado.
- Un matchId o eventId puede reutilizarse en otra combinación de juego/partida.

El productor espera la aceptación del inicio y de todos los eventos anteriores
antes de enviar el cierre. No hay garantías de entrega exactamente una vez,
reintentos idempotentes, persistencia ni reapertura.
El timestamp no decide el orden de ingestión ni el cierre.

## Datos del dominio y propiedad del contrato

Solo los tipos comunes de inicio/cierre tienen efecto de control en Go.
Para ellos Go también admite data arbitrario: no exige players, símbolos,
dimensiones ni resultados. Un evento con points como texto puede almacenarse;
su análisis numérico será inválido en Scala.

[games.md](games.md) es la fuente normativa de datos para Gato, Snake y
carreras. Esos requisitos los consumen perfiles y analizadores Scala,
nunca condicionales por juego en Go. Nuevos juegos pueden adoptar otros
tipos de eventos y objetos data sin cambiar el sobre.

Los ejemplos históricos de arena-combat (DAMAGE_DEALT, PLAYER_ELIMINATED)
siguen siendo eventos admisibles por Go, pero combate no es un analizador
obligatorio ni un juego simulado del MVP seleccionado.

## Snapshots, consulta en vivo y cierre

GET /results/{gameId}/{matchId} toma todos los eventos aceptados hasta un
instante, con state open o closed. Cada consulta solicita POST /analyze.
No hay envío automático al aceptar MATCH_FINISHED, caché ni almacenamiento
de resultados analíticos.

La colección viaja en orden de aceptación. El análisis temporal usa
(timestamp normalizado a UTC, índice original), sin alterar el snapshot.
La política completa de orden, estados analíticos y resultados está en
[metrics-and-patterns.md](metrics-and-patterns.md).
El contrato exterior HTTP se conserva en
[integration-api.md](integration-api.md); no se usa el antiguo status:analyzed.

## Verificación

Los [fixtures](fixtures/analysis-v1/README.md) incluyen snapshots compatibles
con este sobre. Separan validez de ingestión de validez del dominio: los
casos numéricos inválidos deben seguir siendo aceptables por Go.
Scala ya valida los campos consumidos por sus capacidades y el alcance de
tablero documentado. No valida universalmente las reglas de cada videojuego.
