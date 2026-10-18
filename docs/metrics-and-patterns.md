# Resultados, tiempo y patrones — contrato interior v1

Contrato vigente implementado por Scala. El exterior HTTP permanece:
gameId, matchId, state, metrics y patterns; véase [integration-api.md](integration-api.md).
Esta versión interior se identifica por IDs versionados de las capacidades
y la versión del perfil; no se agrega un campo obligatorio exterior.

## Métrica

Ejemplo correspondiente a tres vueltas en un snapshot abierto:

```json
{
  "id": "laps.player",
  "label": "Vueltas por jugador",
  "scope": "player",
  "subjectId": "p1",
  "value": 3,
  "unit": "events",
  "status": "provisional",
  "source": {
    "templateId": "event-metrics/v1",
    "instanceId": "laps",
    "profileVersion": "1.0.0"
  },
  "basis": {"eventCount": 4, "lastEventId": "l3"}
}
```

Todos esos campos son obligatorios. scope=player identifica playerId;
scope=match identifica matchId. La identidad de una fila es
(id, scope, subjectId) dentro de gameId/matchId. id y unit vienen del perfil.
El campo state exterior no se reemplaza por status.

Las capacidades numéricas emiten value numérico o null.
Una especialización puede emitir un objeto documentado: board.result usa
{"outcome":"win|draw|in_progress","winnerId":"ID o null"}.
En empate o progreso winnerId es JSON null, no el texto "null".
No se permiten valores arbitrarios sin contrato del analizador.

source contiene exactamente templateId o analyzerId, más instanceId y
profileVersion. No se inventa un nombre de juego como plantilla.
basis identifica el prefijo de ingestión con eventCount y lastEventId,
incluyendo inicio y cierre. Todos los elementos de una respuesta usan la
misma base. Es una referencia dentro del proceso actual, no un identificador
durable entre reinicios. Sin resultados (juego sin perfil) no hay basis;
no se agrega metadato exterior porque Go lo descartaría.

## Estado analítico

| status | value de métrica | count de patrón | Significado |
| --- | --- | --- | --- |
| provisional | Valor derivado | Cantidad >=0 | Snapshot open con evidencia suficiente; puede cambiar. |
| final | Valor derivado | Cantidad >=0 | Snapshot closed y datos suficientes/válidos. |
| unavailable | null | null | Faltan evidencia o condiciones de disponibilidad. |
| invalid | null | null | Existe un input consumido incorrecto. |

unavailable e invalid requieren reason. Códigos MVP:
awaiting_finish, awaiting_end, missing_end, no_samples,
insufficient_evidence, missing_field, invalid_number, missing_player,
unknown_player, invalid_time, repeated_end, invalid_board, ambiguous_winner.
No se incluye un resultado definitivo con cero como sustituto de evidencia.

Una capacidad availability=live emite provisional en open y final en closed.
availability=final emite unavailable/awaiting_finish en open.
Un dato incorrecto detectado no se oculta como provisional.
El cierre no convierte un promedio sin muestras en 0 ni repara datos inválidos.
Para avg la cantidad de muestras es el denominador: si es cero, no hay valor.
sum sin muestras es 0; count sin muestras es 0.

Los cálculos numéricos utilizan BigDecimal. avg y segundos fraccionarios
se redondean a seis posiciones decimales, HALF_EVEN, al preparar el valor de salida.
No se redondea cada evento antes de sumar. Unidades no se mezclan.
No se serializan NaN ni infinito y no se permite coerción de strings.

## Patrón

```json
{
  "id": "apples.burst",
  "label": "Tres manzanas en diez segundos",
  "scope": "player",
  "subjectId": "p1",
  "status": "final",
  "count": 1,
  "occurrences": [
    {
      "eventIds": ["a1","a2","a3"],
      "startTimestamp": "2026-10-17T18:00:01Z",
      "endTimestamp": "2026-10-17T18:00:11Z"
    }
  ],
  "source": {
    "templateId": "temporal-window/v1",
    "instanceId": "burst",
    "profileVersion": "1.0.0"
  },
  "basis": {"eventCount":9,"lastEventId":"f"}
}
```

id, label, scope, subjectId, status, count, occurrences, source y basis
son obligatorios. count es cantidad de ocurrencias, no cantidad de eventos.
Para un patrón válido sin detecciones count=0, occurrences=[].
Si invalid/unavailable: count=null, occurrences=[], reason obligatorio.
Cada ocurrencia tiene eventIds distintos y tiempos de su primer/último
evento en orden canónico; esos IDs deben pertenecer al snapshot y al sujeto.
El analizador de Gato añade details:{winnerId,cells:[{row,column},...]}.
Su scope es match aunque details identifique al ganador.

La identidad de una ocurrencia es (id, subjectId, eventIds ordenados).
No se generan UUID aleatorios ni se almacenan detecciones entre consultas.
El algoritmo greedy consume descrito en [analysis-templates.md](analysis-templates.md)
impide reutilizar eventos en un mismo patrón. Instancias diferentes pueden
usar los mismos eventos deliberadamente: sus resultados no se suman
automáticamente.

## Orden y datos tardíos

1. Go mantiene orden de aceptación. El índice original es parte del snapshot.
2. Scala no altera la entrada: crea una vista ordenada por instante UTC,
   con desempate por índice original, no eventId ni orden alfabético.
3. Timestamps equivalentes con zonas distintas representan el mismo instante.
4. Los eventos simultáneos tienen un orden determinista de evaluación; no
   demuestran causalidad física. Una terna de ventana admite intervalos de 0 s.
5. Un evento atrasado aceptado mientras open se incluye al siguiente GET.
   Se recalcula toda la colección: una detección provisional puede cambiar.
6. Después de MATCH_FINISHED Go rechaza eventos nuevos. No se supone una
   política de reapertura o corrección de timestamps.
7. duration y ventanas que consumen timestamps rechazan eventos seleccionados
   fuera del intervalo [inicio, cierre] si está cerrado, o anteriores al
   inicio si está abierto: invalid/invalid_time para la salida afectada.
   Conteos sin dependencia temporal pueden seguir siendo válidos.

Para el tablero, turn debe coincidir con la secuencia canónica: un dato
fuera de orden temporal que contradiga el turno produce invalid_board.
No se inventa causalidad para corregirlo.

## Composición y consultas repetidas

Cada capacidad consume eventos de manera independiente y produce filas.
El compositor concatena resultados; nunca acumula la respuesta de un GET
anterior ni usa contadores mutables del juego. IDs repetidos son errores
de configuración. Los arrays se ordenan por (id, scope, subjectId), usando
orden lexicográfico de puntos de código Unicode, independiente de locale.
occurrences conserva orden cronológico del algoritmo.

Una consulta sobre carrera tras dos vueltas devuelve 2; tras la tercera,
3; tras cerrar, 3 final. No requiere cierre para ver contadores.
La duración final permanece unavailable hasta el cierre, pues el protocolo
no incluye observationTime y no se usa la hora de la consulta como evento.

Un GET puede terminar después de que lleguen eventos nuevos o cierre la
partida: state y basis siempre describen el snapshot solicitado.
Consultas concurrentes pueden responder fuera de orden; el cliente debe
serializar su polling o descartar respuestas de prefijos menores mediante
basis. No se promete consistencia entre procesos o reinicios.
Modificar perfiles entre consultas puede cambiar valores finales:
source.profileVersion permite reconocerlo.

## Criterios verificables del motor implementado

- Mismo snapshot/perfil → mismas métricas y patrones, incluso al repetir GET.
- Juego nuevo con perfil existente en capacidades → sin cambiar el ejecutor.
- Un evento alimenta count, sum, avg y ventana sin duplicar la ingestión.
- Ceros no se confunden con evidencia ausente o inválida.
- Partida open sigue admitiendo POST /events después de GET /results.
- Comparar exactamente los objetos esperados de los fixtures; arrays ordenados
  y números por valor, sin exigir el mismo espacio/escape JSON.
