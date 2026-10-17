# Perfiles y capacidades componibles v1

Especificación para el futuro Scala; solo existen documentación y ejemplos.
Un perfil es un dato local asociado exactamente a un gameId, no un programa.
Los perfiles completos están en [examples/profiles](examples/profiles/).

## Elección entre métricas directas y plantillas

A) Una lista plana de métricas es simple para count/sum/avg, pero duplicaría
selección, ventanas y correlaciones al incorporar duración y secuencias.
B) Un sistema de macroplantillas por juego ahorraría líneas, pero escondería
IDs y produciría demasiada abstracción para el MVP.

Se adopta una estructura híbrida: event-metrics/v1 contiene una lista
directa de métricas; duration/v1 y temporal-window/v1 encapsulan mecanismos
reutilizables cuando aportan valor. No hay herencia entre perfiles,
dependencias entre plantillas, expansión recursiva ni expresiones evaluables.
Una plantilla puede instanciarse varias veces con instanceId distintos. availability se aplica a todas las salidas de una instancia; si se necesitan políticas live y final distintas, se declaran instancias separadas con IDs de resultados distintos.

## Perfil versionado

```json
{
  "profileSchemaVersion": 1,
  "profileVersion": "1.0.0",
  "gameId": "example",
  "playersField": "/data/players",
  "capabilities": [
    {
      "templateId": "event-metrics/v1",
      "instanceId": "scores",
      "availability": "live",
      "metrics": [
        {
          "id": "score.player",
          "label": "Puntos",
          "eventType": "SCORED",
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

| Propiedad | Regla |
| --- | --- |
| profileSchemaVersion | Entero 1; cambia al cambiar la estructura del perfil. |
| profileVersion | Versión MAJOR.MINOR.PATCH inmutable; cualquier cambio de perfil obtiene otra versión. |
| gameId | Cadena exacta, única en el registro de perfiles activos. |
| playersField | JSON Pointer opcional al roster en data del único MATCH_STARTED; si se omite, se usan jugadores observados. |
| capabilities | Array, posiblemente vacío, de instancias registradas y tipadas. |
| specializedAnalyzers | Array, posiblemente vacío, de analizadores registrados con parámetros permitidos. |
| instanceId | Cadena única entre capacidades y analizadores de un perfil. |
| availability | live o final; define cuándo puede emitirse un valor, no cambia el snapshot. |

Solo se activa una versión por gameId al arrancar. No hay selección por
gameVersion ni recarga dinámica en el MVP. profileVersion identifica la
semántica analítica, gameVersion la del productor y schemaVersion el sobre.
La selección de versiones por histórico se aplaza: no existe en POST /analyze.

Todos los objetos de configuración rechazan campos desconocidos para
detectar errores tipográficos. IDs y etiquetas son cadenas no vacías.
Las versiones e IDs de plantilla son exactos: un ID desconocido no tiene
fallback. Los IDs de resultados se reservan globalmente en el perfil,
incluyendo métricas y patrones de especializaciones.

## event-metrics/v1

Una instancia requiere metrics no vacío. Cada métrica define id, label,
eventType, operation, scope y unit. scope es player o match.

- count cuenta los eventos seleccionados; prohíbe field, numberKind y minimum.
- sum y avg requieren field bajo /data/, numberKind (integer o decimal)
  y minimum opcional (número). unit declara la unidad del resultado;
  no hay conversiones implícitas.
- Los paths son JSON Pointers de claves de objetos, con escapes ~0 y ~1.
  Sin comodines, acceso a arrays, referencias a resultados, fórmulas ni scripts.
- integer exige un valor JSON numérico matemáticamente entero; decimal admite
  fracciones. Se rechazan strings numéricos, booleanos, null, ausentes y valores
  inferiores a minimum. No se interpreta un texto como número.
- En scope player se emite una fila para cada jugador del roster. Si no hay
  playersField, el conjunto es la unión de playerId no vacío en los eventos
  consumidos por todas las capacidades del perfil, ordenada por ID.
- Si se declara roster: array no vacío, IDs únicos, cadenas no vacías; todo
  playerId de un evento consumido debe pertenecer a él. Si una capacidad
  necesita playerId y falta, sus resultados afectados son invalid.
- Los eventos de partida sin playerId no participan en métricas player,
  salvo que su tipo sea explícitamente seleccionado por una métrica player:
  en ese caso es un input incorrecto, no un jugador ficticio.
- count y sum sin muestras producen 0; avg sin muestras produce null y
  unavailable/no_samples. Se incluyen jugadores con cero contribuciones.
- Un valor incorrecto invalida la métrica para el sujeto afectado; no se
  omite la muestra ni se ofrece un subtotal engañoso. Otro count que solo
  consume el tipo puede seguir siendo correcto.

No se filtran eventos por valores arbitrarios de data en esta versión.
El mismo acontecimiento puede alimentar varias métricas y capacidades.
Cada salida se calcula independientemente; no se suman salidas entre sí.

## duration/v1

Parámetros obligatorios: id, label, scope, startEventType, endEventType, unit.
MVP: startEventType debe ser MATCH_STARTED y unit debe ser seconds.
scope match usa endEventType MATCH_FINISHED; scope player usa un tipo
de evento de jugador configurable (PLAYER_DIED en Snake).

El inicio es único y común; el fin debe ser único por sujeto. Se resta
timestampFin - timestampInicio en segundos, sin usar reloj del servidor.
Los fines repetidos o anteriores al inicio invalidan esa salida.
Un fin de jugador posterior al MATCH_FINISHED también es inválido.
Sin fin: unavailable/awaiting_end en open o unavailable/missing_end en closed.
Una instancia availability:final en open produce unavailable/awaiting_finish.

La duración de partida usa availability:final en los tres perfiles.
El tiempo de vida de Snake usa live: puede conocerse tras PLAYER_DIED
aunque la partida siga abierta, todavía como provisional.
Relacionar inicio, muerte y cierre satisface el requisito multievento.
No se inventa duración para jugadores vivos ni tiempo acumulado de vueltas.

## temporal-window/v1

Parámetros: id, label, eventType, scope=player, size (entero >=2),
windowSeconds (número >=0), overlap=consume.
Solo se implementará esta política y availability=live en el MVP.

Se ordenan eventos por (timestamp UTC, índice de aceptación). Por jugador,
se recorre desde el primer candidato: si los siguientes size eventos
abarcan <= windowSeconds, se emite esa ocurrencia y se consumen todos.
Si no caben, se avanza solo el candidato inicial. Repetir hasta agotar.
No existe reutilización de eventos entre ocurrencias del mismo patrón.
No se enumeran todas las combinaciones posibles de una ventana.

El límite es inclusivo. Tres eventos distintos con igual timestamp
pueden formar una terna; no se afirma causalidad entre ellos.
Cada consulta recalcula desde cero. Eventos tardíos antes del cierre pueden
cambiar incluso las ocurrencias provisionales antes observadas.

## Analizadores especializados

Entrada: snapshot inmutable, eventos ordenados y parámetros validados.
Salida: métricas y patrones tipados; sin efectos laterales ni acceso a Go.
Registro conceptual: Map[AnalyzerId, Analyzer]. El perfil solo refiere IDs
conocidos: no contiene nombres de clases para cargar, URLs de módulos
ni código ejecutable.

tic-tac-toe-line/v1 admite únicamente:
resultMetricId y patternId, ambos no vacíos y reservados en el perfil.
Su contrato es [games.md](games.md). Identifica su instancia, versión del
perfil y snapshot igual que las capacidades genéricas.

## Configuraciones y datos inválidos

| Situación | Política futura |
| --- | --- |
| Perfil JSON inválido, versión no soportada, campo desconocido | Scala no inicia; diagnóstico local con ubicación. |
| Plantilla/analizador desconocido o parámetros incompatibles | Scala no inicia, sin ignorar capacidades. |
| gameId activo duplicado, instanceId repetido o resultado ID duplicado | Scala no inicia. |
| Campo de dominio ausente o tipo incorrecto | HTTP 200 con salidas afectadas invalid y reason; nunca alteración de Go. |
| Roster declarado incorrecto | Scala devuelve 400: no hay sujetos fiables; Go actual lo presenta como 502. |
| Juego sin perfil | HTTP 200 con metrics:[], patterns:[]; sin métricas ficticias. |
| Snapshot exterior incoherente | HTTP 400 de Scala; Go actual lo transforma en 502. |
| Fallo interno de ejecución | HTTP 500 de Scala; Go actual lo transforma en 502. |

Un error compartido puede invalidar todas las salidas dependientes, pero
una métrica válida no depende del éxito de otra. No hay éxitos parciales
ocultos ni un motor que ejecute configuraciones inválidas.

## Prioridades

| Indispensable MVP | Ampliación sencilla posterior | Complejidad fuera del MVP |
| --- | --- | --- |
| count/sum/avg, agrupación player/match | min/max para mejor vuelta | Reglas universales, DSL e intérprete |
| duration inicio-fin | nuevas unidades con conversión explícita | Torneos, equipos y rondas anidadas |
| Ventana de cardinalidad sin solapamiento | más mapeos de eventos a las mismas capacidades | Joins generales y procesamiento incremental |
| Perfil que compone capacidades; tablero Gato | count para turnos/rondas/vueltas ya sirve si el productor emite su fin | Validación completa de ajedrez/damas |

Daño y puntuaciones no necesitan una plantilla propia si son sumas.
Vueltas y turnos pueden ser conteos. No se promete inferir sus finales
desde física del juego: el productor debe emitirlos.
