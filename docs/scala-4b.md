# Implementación Scala 4B — patrones y analizadores

> Documento histórico del cierre de 4B. Sus resultados y exclusiones describen
> esa etapa, no el estado actual: simulador, Docker, CI y distribución se
> completaron después. Ver [índice vigente](README.md) y [auditoría](academic-audit.md).

Estado: temporal-window/v1 y tic-tac-toe-line/v1 implementados, registrados
y verificados. Los 13 fixtures aprobados producen sus respuestas completas
en el motor puro, por HTTP real y mediante el puente Go → Scala real.
La política transitoria 501 de 4A fue eliminada.
No se modificaron Go, perfiles ni fixtures.

## Arquitectura y archivos

- TemporalWindow.scala: capacidad reutilizable, independiente del juego.
- TicTacToeLine.scala: analizador especializado y registro por analyzerId.
- Model.scala: AnalyzerConfig, procedencia tipada, valores estructurados,
  detalles de ocurrencias y composición Outputs.
- Engine.scala: registros de capacidades/analizadores y combinación ordenada
  de métricas y patrones. No contiene condicionales por gameId.
- Decoding.scala: registro de analizadores; conserva además el timestamp
  textual para referenciar los tiempos originales de las ocurrencias.
- HttpService.scala y Main.scala: eliminan el rechazo/log transitorio 501.
- Suites existentes: comparan todos los fixtures completos, no respuestas parciales.
- PatternsSuite.scala: casos límite, extensibilidad y concurrencia.
- scripts/check-go.py: prueba de contrato con los procesos reales,
  reutilizando los fixtures; no es un simulador.

La extensión conserva las métricas numéricas y sus cálculos decimales exactos.
Una métrica puede tener un número o un objeto estructurado, nunca ambos.
source emite exactamente templateId o analyzerId, con instanceId y
profileVersion. Se conserva el exterior HTTP sin campos nuevos obligatorios.

## Ventana funcional genérica

Se selecciona el tipo configurado, se agrupa por jugador y se crea una vista
ordenada por (instante UTC, índice original de aceptación).
No se altera el snapshot ni se usa eventId como desempate.

El recorrido es tail-recursive. Compara directamente el primero y el N-ésimo
candidato usando aritmética temporal exacta de nanosegundos, sin redondear la
ventana a seis decimales. Si caben en el límite inclusivo, emite una ocurrencia
y avanza N posiciones. Si no caben, avanza una. Solo materializa el grupo al
aceptarlo: no enumera combinaciones ni reutiliza eventos en ese patrón.

Los eventIds y timestamps corresponden a acontecimientos realmente recibidos.
Los offsets equivalentes se comparan como el mismo instante, conservando su
texto original en la salida. Un evento tardío antes del cierre puede cambiar
las ocurrencias provisionales: cada consulta recalcula desde cero.

Los campos points no alimentan la ventana; un valor numérico inválido no
invalida por sí solo la evidencia temporal. Jugadores o tiempos inválidos
sí producen invalid con count:null, reason y occurrences:[].
Con menos de N candidatos válidos hay cero detecciones, no una terna ficticia.
El DSL aprobado permite availability:live para ventanas: provisional en
open, final en closed. La infraestructura distingue unavailable para
resultados aún no disponibles sin habilitar nuevas opciones del perfil.

Costo: ordenación O(n log n), recorrido posterior lineal por grupo.
Se mantienen los límites operativos y de respuesta de 4A.

## Analizador de tablero

Se registra como tic-tac-toe-line/v1; cualquier perfil puede seleccionarlo
sin modificar el ejecutor común. La configuración conserva solo los IDs
documentados de resultado y patrón.

Reconstruye con foldLeft y estado inmutable: celdas ocupadas y primera
victoria observada. Verifica dos jugadores únicos, asignación exacta X/O,
coordenadas, turnos consecutivos en orden canónico, correspondencia
jugador/símbolo y ausencia de casillas ocupadas.
Comprueba todas las ocho líneas y continúa verificando el resto del snapshot
después de una victoria para detectar datos inválidos o dos ganadores.

La primera línea se conserva. Si un movimiento completa varias, el orden
es filas, columnas, diagonal principal y secundaria. Una ocurrencia incluye
los tres eventos de esa línea, sus tiempos y details con ganador y celdas.

Un ganador ambiguo invalida ambas salidas especializadas. Los conteos
genéricos siguen describiendo eventos aceptados, no movimientos legales.
Una línea produce win; tablero completo sin línea produce draw.
Un tablero incompleto abierto produce in_progress provisional.
Si está cerrado e incompleto, board.result es unavailable/insufficient_evidence;
el patrón conserva cero líneas observadas, final. No se inventa un empate.

No valida alternancia, obligación de empezar con X ni detención inmediata
tras ganar. No implementa todas las reglas posibles de Gato. Las pruebas
comprueban expresamente este límite.

## Evidencia ejecutada

| Verificación | Resultado |
| --- | --- |
| Compilación Scala | Correcta |
| Todas las pruebas Scala | 143 aprobadas, 0 fallos |
| Motor puro contra expected completos | 13/13 |
| HTTP Scala contra expected completos | 13/13 |
| HTTP concurrente multijuego | Dos consultas concurrentes de cada fixture, respuestas completas estables |
| Pruebas Go y go vet | Correctas |
| Oráculo documental | 30 JSON, 13 pares, 9 configuraciones inválidas |
| Go → Scala real | 61 eventos, 10 partidas, 26 consultas secuenciales y 20 concurrentes |
| Go, perfiles y fixtures aprobados | Sin cambios |

La comparación incluye métricas/patrones, valores estructurados, sujetos,
estados, unidades, source, profileVersion, basis, reason y occurrences/details.
No se limita a la sintaxis JSON ni a un subconjunto de campos.

Los fixtures son ttt-win, ttt-draw, ttt-open, snake-positive, snake-negative,
snake-invalid-number, snake-missing-points, snake-tied, race-start,
race-live-2, race-live-3, race-final y unknown-game. No quedan fixtures
analíticos pendientes de 4A/4B.

Los casos adicionales incluyen límite exacto y un nanosegundo fuera,
solapamientos, 9000 candidatos, zonas equivalentes, eventos simultáneos/tardíos,
datos numéricos inválidos independientes, las ocho líneas, desempate entre
líneas, turnos/coordenadas inválidos, ocupación repetida, símbolos y roster
incorrectos, ganadores ambiguos, partidas parciales y repetición/concurrencia.

## Reproducir

Desde scala-analysis:

~~~powershell
.\sbtw.ps1 compile test
.\sbtw.ps1 run
~~~

Scala escucha por defecto en 127.0.0.1:9000, configurable como en
[su README](../scala-analysis/README.md). En otra terminal, arrancar un
proceso Go nuevo con SCALA_SERVICE_URL=http://localhost:9000.

Con ambos servicios reales disponibles y Go sin partidas previas,
desde la raíz del repositorio, con Python 3.10+:

~~~sh
python scala-analysis/scripts/check-go.py --evidence-dir scala-analysis/.tools/integration-4b
~~~

La prueba envía los fragmentos JSON originales para no alterar números ni
timestamps. Reutiliza prefijos de carreras sin reingerir eventos duplicados.
Consulta cada fixture dos veces antes de avanzar su partida y después
consulta concurrentemente el último snapshot de cada partida.
No compara un prefijo abierto antiguo contra una partida ya cerrada.
Guarda respuestas y summary.json solo si se indica evidence-dir, ignorado
por Git en el ejemplo.

El script no inicia ni detiene servicios ajenos; requiere arrancarlos.
La verificación de esta entrega inició procesos propios independientes
de Go y Scala y los detuvo al terminar. No utilizó mocks.

## Restricciones y siguiente tarea

Se mantienen memoria local en Go, análisis por snapshot completo, polling
mediante GET y ausencia de persistencia, caché, push y reintentos idempotentes.
Las ventanas y el tablero no validan reglas físicas completas del juego.
No hay simulador, Docker, CI/CD, deployment ni nuevas dependencias en 4B.

La siguiente tarea es un simulador multijuego determinista: emitir eventos
de Gato, Snake y racing con partidas intercaladas, esperar sus aceptaciones
y consultar resultados abiertos/finales. Debe reutilizar los contratos,
incluir casos válidos e inválidos y no convertirse en videojuegos ejecutables.
