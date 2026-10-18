# Estado y hoja de ruta de GameStats

Corte documental: 18 de octubre de 2026. La [auditoría](academic-audit.md)
identifica fuentes, evidencias y limitaciones; no sustituye la evaluación docente.

## Estado verificado

| Etapa | Estado presente | Evidencia |
| --- | --- | --- |
| Go 1–3 | Implementado | Sobre v1, memoria por (gameId, matchId), mutexes, lifecycle, snapshots y HTTP a Scala. |
| Scala 4A | Implementado | Perfiles, motor puro count/sum/avg, duración y composición. [Informe histórico](scala-4a.md). |
| Scala 4B | Implementado | Ventanas y tablero; 143 pruebas Scala y 13 fixtures en CI. [Informe histórico](scala-4b.md). |
| Simulador 5 | Implementado | Seis escenarios propios, 50 eventos y 60 consultas por demo. [Manual](../simulator/README.md). |
| Docker 6 | Implementado y verificado | Tres imágenes, servicios healthy, red interna y limpieza. [Guía](docker.md). |
| CI 7 | Aprobada | [Run 39104918511](https://github.com/JusMH/gamestats/actions/runs/39104918511), commit 7782fa9b4c88a73317a3b0b1eb51d0fbacca5d15; ambos detectores Go race. |
| Distribución 8 | Publicada y evaluación aprobada; automatización parcial | [v1.0.0](https://github.com/JusMH/gamestats/releases/tag/v1.0.0), imágenes públicas, ZIP evaluado. Finalize falló y requirió recuperación manual. |
| Deployment académico local | Ejecutado con Compose | Evaluación anónima desde ZIP, servicios reales y simulador; Linux/amd64. |
| Deployment remoto | No implementado | Fuera de la modalidad elegida; no hay alojamiento en nube. |

El enunciado exige un entorno de deployment definido por el equipo, no impone
un proveedor remoto. Se eligió Docker Compose local: compilación reproducible,
contenedores, CI, publicación GHCR, distribución por Release e instalación
verificada son pasos distintos. El fallo real de Finalize impide presentar el
workflow de publicación completo como verde, aunque el paquete evaluado sí existe.

## Próxima tarea necesaria: reparar Finalize en un cambio separado

1. Reproducir el fallo de confirmación de borrador observado en la ejecución
   [39105352591, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2):
   `Draft creation not confirmed; retry failed job`.
2. Corregir y probar la reconciliación del borrador mediante identificadores de
   GitHub, incluyendo respuesta perdida, tag provisional y reanudación segura.
   La causa interna exacta aún requiere esa investigación; no se cambia código
   durante la auditoría documental.
3. Mantener aprobación humana, CI exacta, digests y evaluación antes de publicar.
   Crear una nueva versión solo después de revisión y autorización. No mover,
   reconstruir, sobrescribir ni volver a finalizar v1.0.0 ciegamente.
4. Comprobar que el workflow completo termina success y que los adjuntos
   publicados son los mismos que pasaron evaluación. Las pruebas locales
   sintéticas no bastan para certificar este paso remoto.

## Consolidación documental y siguientes decisiones

Esta revisión corrige el estado presente, conserva informes históricos y añade
una guía para conectar juegos externos. No modifica v1.0.0 ni su ZIP. Una futura
versión documental puede incorporar estas correcciones y retirar el comentario
obsoleto de la plantilla Compose de evaluación, en una tarea separada revisada.

La integración con juegos cuya IA use Prolog es **FUTURA**; ver
[integrar nuevos juegos](integrating-new-games.md). No se implementa aquí.

Antes de escalar se necesitan mediciones y decisiones de retención/límites:
Go almacena sin límite global, Scala analiza como máximo 10 000 eventos y
8 MiB por petición. No se prometen prestaciones no medidas. Permanecen fuera
del alcance persistencia distribuida, Kafka, motor universal, ajedrez/damas
completos, GUI, equipos/torneos, procesamiento incremental y WebSockets/SSE.
min/max tampoco forma parte de las capacidades actuales. Nuevos mapeos a
count/sum/avg ya pueden incorporarse mediante perfiles, con reinicio de Scala.
