# Juegos de demostración y contratos de data

Estado: contratos para implementar Scala y el simulador en etapas futuras.
Go solo conoce el [sobre v1](events.md) y los tipos comunes de control.
Los datos aquí exigidos no se convierten en requisitos de ingestión Go.

## Datos compartidos por los perfiles de demostración

MATCH_STARTED omite playerId y contiene {"players":["p1","p2"]}.
players es un array no vacío de IDs únicos. Las capacidades player utilizan
ese roster para emitir también ceros y valores no disponibles.
MATCH_FINISHED omite playerId y admite data:{}; cierra el flujo, pero no
declara ganador ni puntuación. No se confía en resultados que reporte el
productor: se derivan solamente cuando hay evidencia.
Los campos adicionales de data se permiten y no se interpretan.

Los tres perfiles MVP usan estos datos comunes por conveniencia; otro juego
puede no declarar roster y usar los jugadores observados según su perfil.

## Gato / Tic-tac-toe

gameId: tic-tac-toe.
Perfil: [tic-tac-toe.json](examples/profiles/tic-tac-toe.json).

| Evento | playerId | Campos consumidos en data |
| --- | --- | --- |
| MATCH_STARTED | Omitido | players: exactamente dos IDs; symbols: objeto ID → símbolo, exactamente X y O, una asignación por jugador. |
| MOVE_PLACED | Jugador del roster | turn: entero positivo; row y column: enteros de 0 a 2; symbol: X u O, acorde a symbols. |
| MATCH_FINISHED | Omitido | {}. No exige outcome ni winnerId. |

Ejemplo de inicio:

```json
{"players":["alex","bea"],"symbols":{"alex":"X","bea":"O"}}
```

Ejemplo de movimiento (dentro del sobre, playerId=alex):

```json
{"turn":1,"row":0,"column":0,"symbol":"X"}
```

Las coordenadas son base cero: row=0 es la fila superior, column=0 la
columna izquierda. No se incluyen dibujos ni estado completo del tablero.

### Métricas y especialización

event-metrics/v1 cuenta MOVE_PLACED por jugador y partida.
duration/v1 resta inicio y cierre.
tic-tac-toe-line/v1 reconstruye una cuadrícula inicialmente vacía mediante
fold sobre el orden temporal canónico. Verifica:

- roster y asignación de símbolos válidos;
- turn exactamente 1,2,... en el orden canónico;
- coordenadas enteras dentro de 3x3;
- playerId/symbol acordes al inicio;
- ausencia de casillas ocupadas;
- inexistencia de líneas ganadoras de jugadores distintos.

Detecta filas, columnas y diagonales. Emite solo la primera línea ganadora;
si un mismo movimiento completa varias, el desempate es filas (0..2),
columnas (0..2), diagonal principal y secundaria.
Cada ocurrencia incluye los tres eventIds de la línea ordenados temporalmente,
sus celdas y winnerId. No cuenta cada prefijo posterior como otra victoria.

Una línea permite outcome=win. Nueve celdas válidas y ninguna línea permiten
outcome=draw. Sin eso: in_progress en open; unavailable/insufficient_evidence
en closed, nunca empate automático porque llegó MATCH_FINISHED.

**Alcance:** comprueba evidencia de una línea, no la legalidad absoluta.
No certifica que X haya empezado, alternancia de jugadores, conteos legales
de X/O ni que se haya detenido el juego inmediatamente tras ganar.
No reconstruye movimientos ausentes. Si hay dos ganadores, hay ambigüedad
y las salidas del analizador son invalid/ambiguous_winner.
Un error de tablero invalida resultado y patrón del analizador; los conteos
de MOVE_PLACED siguen describiendo eventos aceptados, no movimientos legales.
No se propone soporte completo de ajedrez o damas.

## Snake

gameId: snake. Perfil: [snake.json](examples/profiles/snake.json).

| Evento | playerId | data |
| --- | --- | --- |
| MATCH_STARTED | Omitido | players: lista de jugadores. |
| APPLE_EATEN | Jugador del roster | points: entero no negativo, incremento otorgado por esa manzana. |
| PLAYER_DIED | Jugador del roster | cause: wall, self_collision u other, dato descriptivo para futuras reglas. |
| MATCH_FINISHED | Omitido | {}. |

points nunca es puntuación acumulada. Su suma es la puntuación derivable;
el promedio es suma de points / cantidad de APPLE_EATEN de ese jugador.
No se inventan penalizaciones, bonus ni puntos por sobrevivir.
No hay un evento separado SCORE_CHANGED que pueda duplicar el incremento.

El perfil compone:
- Conteos de manzanas por jugador y partida; suma y promedio de puntos.
- Conteo de muertes por jugador.
- Duración inicio → cierre por partida.
- Tiempo inicio → muerte por jugador; sigue no disponible sin muerte.
- Tres manzanas del mismo jugador en <=10 segundos, consumiendo ternas.

Cada jugador debe tener como máximo un PLAYER_DIED para interpretar un
tiempo de vida; repeticiones con eventId distinto invalidan esa duración.
La duración comprueba muerte >= inicio y <= cierre si existe.
El patrón no depende del campo points; un points inválido no falsifica
que se recibieron tres acontecimientos APPLE_EATEN.

No se simulan posiciones, velocidad, cuerpo de la serpiente ni colisiones.
cause no se verifica físicamente ni altera las métricas MVP.
Tampoco se certifica que no haya manzanas después de una muerte; esa
validación de legalidad requeriría una regla especializada posterior.
Los datos normales del simulador futuro respetarán ese orden.
Los campos requeridos por una salida se validan antes de calcularla.

## Carreras: prueba de extensibilidad

gameId: racing. Perfil: [racing.json](examples/profiles/racing.json).

| Evento | playerId | data |
| --- | --- | --- |
| MATCH_STARTED | Omitido | players. |
| LAP_COMPLETED | Jugador del roster | lap: entero positivo informativo; durationMs: entero positivo, duración de esa vuelta en milisegundos. |
| MATCH_FINISHED | Omitido | {}. |

El productor, no Go ni Scala, determina que terminó una vuelta.
Las capacidades MVP consumen durationMs y tipo de evento; lap sirve al
productor y futuras reglas, no valida orden ni unicidad de vueltas todavía.
El perfil declara minimum=1 para durationMs. Una vuelta repetida con otro
eventId se cuenta dos veces: no se promete deduplicación semántica por lap.

event-metrics/v1 produce vueltas por jugador/partida, suma y promedio de
durationMs por jugador. duration/v1 produce tiempo inicio → cierre.
Ambas capacidades coexisten sin analizador especializado de carreras.
Mejor tiempo requiere min, una extensión sencilla **no incluida en el MVP**.

[Los fixtures race-*](fixtures/analysis-v1/README.md) consultan inicio, dos
vueltas, tres vueltas y cierre. Los contadores son 0,2,3,3; jamás 0,2,5,8.
La duración de partida (32 s) no se confunde con suma de vueltas (30000 ms).

## Juego desconocido

Go lo admite si el sobre y el ciclo de vida son válidos.
Scala sin perfil devuelve los mismos gameId/matchId/state y arreglos vacíos.
No hay fallback de estadísticas específicas ni registro dinámico por API.
Añadir un perfil permite métricas sencillas sin cambiar Go o el ejecutor Scala.
