# MVP académico y siguiente implementación

## Estado verificable de esta etapa

Go ya tiene ingestión, validación v1, estado concurrente, snapshots y puente
HTTP bajo demanda. Esta entrega agrega únicamente diseño documental,
perfiles JSON, fixtures y un verificador de ejemplos.
No implementa Scala, simulador, lógica Go adicional, Docker ni CI/CD.

## Primera implementación obligatoria del sistema completo

| Requisito | Evidencia prevista |
| --- | --- |
| Go: flujo, memoria y concurrencia | Pruebas existentes de ingestión, partidas y snapshots. |
| Scala HTTP real independiente | POST /analyze probado con el puente Go. |
| Programación funcional | Funciones puras con filter/map/groupBy/fold/flatMap; errores explícitos. |
| count/sum/avg configurables | Perfiles y fixtures Snake/racing, sin condicionales por juego. |
| Composición y alcances player/match | Perfiles con varias métricas y capacidades simultáneas. |
| Métrica multievento | Duración inicio-fin y tiempo inicio-muerte; no solo campos precalculados. |
| Secuencia | Ventana Snake de tres manzanas y reconstrucción de tablero Gato. |
| Múltiples juegos | Snake y Gato principales; racing prueba de extensibilidad. |
| Análisis en vivo y final | race-start → race-live-2 → race-live-3 → race-final, GET repetidos. |
| Simulación suficiente | Futuro productor determinista de eventos, partidas intercaladas; ningún videojuego completo. |
| Pruebas de contrato e integración | Requests/expected de fixtures, errores y concurrencia. |
| Docker, CI/CD y deployment | Etapas posteriores obligatorias de la entrega académica. |
| Documentación y defensa | Documentos de esta carpeta y resultados reproducibles. |

Las plantillas indispensables son event-metrics/v1, duration/v1,
temporal-window/v1 y la especialización tic-tac-toe-line/v1.
No es necesario crear un subsistema distinto para cada sustantivo:
puntajes/daño usan sum; turnos/rondas/vueltas usan count si hay eventos.

## Extensiones sencillas posteriores, no implementadas

min/max para mejor vuelta, unidades adicionales explícitas, nuevas
configuraciones sobre tipos existentes. Su relativa simplicidad no las
incluye automáticamente en el primer motor.
Nuevos mapeos de eventos a count/sum/avg ya son configuración MVP.

## Extensiones opcionales excluidas

Ajedrez/damas completos, motor universal de reglas, interfaz gráfica,
registro dinámico mediante API, persistencia distribuida, Kafka, torneos,
equipos, procesamiento incremental avanzado, WebSockets/SSE.
No se autorizan en esta etapa ni se añaden para cumplir el MVP.
La retención básica deberá decidirse antes de escalar el despliegue,
sin introducir por ello una base de datos distribuida.

## Próxima tarea precisa: Scala mínimo con capacidades genéricas

1. Preparar scala-analysis como módulo ejecutable con dependencias mínimas
   de HTTP/JSON seleccionadas explícitamente; no cambiar Go.
2. Implementar modelos del sobre y snapshot y POST /analyze con exterior
   exacto de integration-api.md. Scala no mantiene estado entre solicitudes.
3. Cargar perfiles locales una vez al arrancar, validarlos y fijar versión.
   Registrar capacidades por ID; rechazar configuraciones desconocidas.
4. Implementar funciones puras para event-metrics/v1 (count/sum/avg) y
   duration/v1, validación numérica Decimal, roster, estados y composición.
5. Hacer pasar primero race-start, race-live-2, race-live-3, race-final y
   unknown-game; consultas repetidas no cambian valores ni cierran partidas.
6. Añadir pruebas de configuración inválida, IDs repetidos, campos ausentes,
   división sin muestras, números incorrectos y snapshots incoherentes.
7. Conectar el servicio real mediante SCALA_SERVICE_URL y ejecutar pruebas
   de integración con Go existente; conservar 200/502/503/504 actuales.
8. Documentar instalación y ejecución Scala y la evidencia de compatibilidad.

Criterio de salida de esa tarea: racing demuestra dos capacidades
componibles con valores en vivo/finales, sin un RacingAnalyzer ni if gameId.
No implementar simulador, Docker o UI dentro de esa tarea.
Después, en tareas acotadas: ventana temporal Snake, analizador Gato,
simulador de tres juegos, integración completa, Docker, CI/CD y despliegue.
El MVP académico no estará completo hasta esas etapas.

## Riesgos y decisiones a ratificar

- Este diseño fija políticas para no dejar ambigüedad: ternas greedy sin
  solapamiento, orden por timestamp/índice, perfil estático por gameId,
  error por muestra inválida, ausencia de perfil → arrays vacíos.
  El equipo debe ratificarlas antes de implementar; no bloquean esta
  entrega documental ni cambian el transporte existente.
- Alcance limitado de Gato: demostrar una línea no demuestra una partida
  legal completa. Decidir si la rúbrica exige validación adicional.
- El puente oculta errores HTTP de dominio de Scala como 502. Se conserva;
  si se desea diagnóstico público más preciso hará falta otra tarea Go.
- Go valida solo el exterior analítico. Confiar en interiores exige pruebas
  y validación Scala; su ausencia no se resuelve editando Go en esta etapa.
- Los perfiles no viajan en la petición. Cambiar la versión activa cambia
  resultados incluso de partidas cerradas. Versiones inmutables y source
  permiten identificarlo, pero no hay análisis histórico por versión.
- Polling y snapshots completos pueden exceder 5 s/8 MiB en partidas largas.
  Medir antes de elegir intervalos, límites globales y retención.
- Confirmar herramienta/build y bibliotecas Scala según entorno académico,
  más plataforma y credenciales de despliegue, en sus tareas respectivas.
- No hay garantías exactamente una vez, datos durables ni estado compartido.
