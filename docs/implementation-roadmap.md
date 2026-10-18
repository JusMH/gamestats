# Estado y hoja de ruta de GameStats

Estado documentado de la versión revisada: 18 de octubre de 2026.
El enunciado y su aceptación corresponden al docente; la documentación distingue
las verificaciones ejecutadas de las decisiones pendientes.

## Versión publicada v1.0.1

| Área | Estado | Evidencia |
| --- | --- | --- |
| Go 1–3 | Implementado | Sobre v1, partidas concurrentes, snapshots y puente HTTP. |
| Scala 4A/4B | Implementado | Motor funcional, métricas, ventanas, tablero y 143 pruebas en CI. |
| Simulador 5 | Implementado | Seis escenarios, 50 eventos y 60 consultas por demostración. |
| Docker 6 | Verificado | Tres imágenes, servicios healthy y pruebas integradas. |
| CI | Aprobada para el commit publicado | [39114936260](https://github.com/JusMH/gamestats/actions/runs/39114936260), con los seis trabajos correctos. |
| Distribución | Publicada con automatización completa | [39115113580](https://github.com/JusMH/gamestats/actions/runs/39115113580), incluida finalización. |
| Deployment | Local con Compose | [Release v1.0.1](https://github.com/JusMH/gamestats/releases/tag/v1.0.1), con imágenes y ZIP evaluados. |

Commit publicado: `2d32836a8da097de9a43c431d79a2bcea675c5cf`.
La reparación de Finalize se incorporó después de la auditoría histórica de
v1.0.0. No figura como tarea actual. La [auditoría](academic-audit.md) conserva
el fallo y la recuperación de aquella versión, sin reescribir su evidencia.

## Ajustes posteriores en esta rama

1. Referencias de instalación y estado actualizadas a v1.0.1.
2. Documentación separada del historial de v1.0.0 y del estado posterior.
3. Ruta sin tildes documentada para construir con Docker en este equipo.
4. Decodificación UTF-8 explícita del JSON de Compose en las pruebas.
5. [Guía CMD](windows-cmd.md) y lanzador Scala equivalente para Windows.
6. Informes limitados al actual y dos anteriores por destino estable `--report`.

No se publicará otro paquete automáticamente por estos cambios. Un PR hacia
main dispara CI; publicar una nueva versión sigue requiriendo decisión humana
y el workflow manual. Los artefactos v1.0.1 conservan sus contenidos originales.

## Alineación de límites implementada

Go limita la partida a 10 000 eventos y 8 MiB serializados, igual que Scala.
Reserva un evento y 1 MiB para el cierre y rechaza excesos con 413 sin mutación.
Las comprobaciones se realizan bajo el bloqueo de la partida; se prueban las
fronteras exactas, la concurrencia y el análisis abierto/cerrado con Scala real.
Consultar [política HTTP](integration-api.md#capacidad-y-reserva-para-cerrar) y
[verificación de cada entregable](verificacion-entrega.md).

## Alcance conservado

No se añaden persistencia, retención automática de partidas, caché, push,
alojamiento remoto, nuevos juegos, reglas universales ni base de datos.
La retención de informes del simulador no cambia la memoria de Go ni el
análisis Scala. La posible integración Prolog sigue siendo futura.
