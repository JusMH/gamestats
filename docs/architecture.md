# Arquitectura genérica de GameStats — diseño de Etapa 1

Decisión de diseño para implementar; no representa un motor Scala existente.
Go ya implementa ingestión, estado concurrente, snapshots y cliente HTTP.
Scala real, simulador, Docker, CI/CD y deployment siguen pendientes.

## Flujo real y objetivo

```text
Productor externo o futuro simulador
  -- POST /events --> Go (sobre v1, duplicados, ciclo de vida, memoria)
  <-- accepted/rejected

Cliente o videojuego
  -- GET /matches/{gameId}/{matchId} --> resumen de ingestión
  -- GET /results/{gameId}/{matchId} --> Go copia snapshot open/closed
                                       |
                                       +-- POST /analyze --> futuro Scala
                                       |                    perfil + capacidades
                                       |                    + analizador opcional
                                       <-- métricas/patrones
  <-- respuesta del snapshot solicitado (sin caché)
```

El productor detecta acontecimientos: por ejemplo, que terminó una vuelta.
GameStats analiza esos eventos; no ejecuta Snake, Gato ni una consola.
Cerrar una partida NO dispara una llamada a Scala.
El cliente puede consultar tras un evento relevante o mediante polling.
Sin una nueva solicitud no hay actualización automática, WebSockets ni SSE.

## Tres niveles

| Nivel | Responsabilidad | Extender |
| --- | --- | --- |
| A: núcleo genérico | Go coordina eventos; Scala recibe snapshots y compone resultados sin identificar juegos mediante if/switch. | Nuevo gameId no cambia Go. |
| B: capacidades reutilizables | event-metrics/v1, duration/v1, temporal-window/v1 seleccionadas y parametrizadas por perfiles declarativos. | Nuevas métricas count/sum/avg: solo configuración. |
| C: especialización | Funciones registradas por ID, p. ej. tic-tac-toe-line/v1. | Agregar una implementación y registrarla; no cambiar el ejecutor común. |

Cada perfil selecciona múltiples capacidades. Racing combina métricas de
vueltas con duración; Snake combina métricas, duración y ventana temporal.
Gato combina movimientos, duración y analizador de tablero.
Ninguna plantilla implica que su juego asociado sea el único compatible.

## Responsabilidades y límites

Go conserva los eventos en memoria por clave estructurada (gameId, matchId).
Un RWMutex protege el registro y otro cada partida. GetEvents copia estado,
eventos y bytes de data bajo el bloqueo de lectura; libera los bloqueos
antes de serializar o usar la red. Las llamadas a Scala no bloquean la
ingestión mientras esperan respuesta.

Scala validará el snapshot, resolverá el perfil, ejecutará funciones puras
sobre colecciones inmutables y compondrá métricas y patrones. No agregará
eventos, no decidirá si Go acepta una solicitud ni almacenará estadísticas
acumuladas entre GET. La validez analítica no equivale a aceptación en Go.

Cada consulta recalcula el snapshot completo. Repetir una consulta con
idénticos eventos, orden y perfil produce el mismo JSON semántico. Al entrar
nuevos eventos cambian los resultados provisionales. El cierre fija los
eventos, pero solo la misma versión del perfil garantiza reproducibilidad.
El diseño no ofrece procesamiento incremental ni resultados globales entre
partidas. "Multijuego" significa perfiles distintos en el mismo servicio,
con respuestas por una sola partida.

## Diseño funcional futuro de Scala

| Paquete conceptual | Función |
| --- | --- |
| model | Evento genérico, Snapshot, Profile, Metric, Pattern; JSON de data preservado. |
| validation | Sobre/snapshot y errores de datos consumidos por una capacidad. |
| profiles | Cargar y validar perfiles locales versionados al arrancar. |
| templates | Registro inmutable ID → capacidad tipada y validación de sus parámetros. |
| metrics | count/sum/avg con números decimales, sin float binario como fuente de verdad. |
| analyzers | Registro inmutable de funciones especializadas; tablero de Gato. |
| composition | Expandir perfiles, detectar IDs repetidos y ordenar resultados. |
| http | POST /analyze; serialización, límites y manejo de errores. |

No se impone framework ni biblioteca de JSON/HTTP en esta etapa.
La implementación Scala decidirá dependencias mínimas al preparar su módulo.

filter selecciona eventos por tipo; groupBy agrupa jugadores; map transforma
campos validados; fold calcula acumuladores y reconstruye el tablero;
flatMap reúne salidas de las capacidades; collect extrae casos tipados
después de validar. collect no debe descartar números inválidos
silenciosamente. avg se calcula con suma y cantidad de muestras.
Los errores se representan como valores (Either/Validation conceptual),
sin excepciones como flujo normal. No hace falta un framework de plugins.

El ejecutor resuelve IDs del perfil en registros; no utiliza condicionales
por gameId. Una nueva especialización requiere compilar Scala, pero no
reescribir el núcleo. No se carga código desde perfiles.

## Agregar un juego y componer capacidades

1. Definir sus eventos data, unidades y productor con el sobre v1.
2. Añadir un perfil local con gameId y versión; usar tantas instancias
   de capacidades como necesite.
3. Asegurar IDs únicos en métricas y patrones y declarar inputs precisos.
4. Si una regla no cabe en las capacidades MVP, decidir si es necesaria:
   registrar un analizador especializado o aplazarla.
5. Añadir snapshot, salida esperada y casos abiertos/cerrados.
6. Validar perfiles antes de iniciar Scala y ejecutar pruebas de contrato.

El [perfil racing](examples/profiles/racing.json) usa solo capacidades
reutilizables: ninguna lógica de carreras debe añadirse al núcleo.
Para una nueva suma, se agrega otra definición a event-metrics/v1, no código.

## Documentos normativos

- [events.md](events.md): sobre y ciclo de vida Go.
- [integration-api.md](integration-api.md): transporte HTTP existente.
- [analysis-templates.md](analysis-templates.md): perfiles y capacidades.
- [games.md](games.md): datos de demostración.
- [metrics-and-patterns.md](metrics-and-patterns.md): resultados, tiempo y reglas.
- [fixtures](fixtures/analysis-v1/README.md): casos y derivaciones comprobables.
- [implementation-roadmap.md](implementation-roadmap.md): MVP y siguiente tarea.

## Riesgos

Memoria y copias crecen con eventos acumulados; no hay retención ni partición
distribuida. Cada polling recalcula todo y consume presupuesto de 5 s.
Respuestas mayores de 8 MiB fallan en el puente Go. Estado y resultados
pueden cambiar entre solicitudes: el snapshot es consistente, no instantáneo
al recibirlo. No existe un reloj de simulación en el protocolo, por lo que
no se calcula "duración hasta ahora" a partir del reloj del servidor.
