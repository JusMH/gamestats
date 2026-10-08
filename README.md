# GameStats

Plataforma multilenguaje para el procesamiento y análisis de eventos de videojuegos. Proyecto académico del curso de Lenguajes de Programación.

## Propósito

Recibir eventos de múltiples partidas y juegos, organizarlos y producir estadísticas y detecciones de patrones sin depender de la implementación de un videojuego específico.

## Componentes previstos

| Carpeta | Responsabilidad |
| --- | --- |
| `go-ingestion/` | Recepción, validación, agrupación, estado temporal y concurrencia en Go. |
| `scala-analysis/` | Transformación funcional, estadísticas y patrones en Scala. |
| `simulator/` | Fuente simulada de eventos para pruebas y demostración. |
| `docs/` | Arquitectura, contrato de eventos y decisiones técnicas. |
| `.github/workflows/` | CI/CD (por implementar cuando existan los módulos). |

## Estado del proyecto

**Fase inicial: estructura y documentación.** Todavía no hay servicios ejecutables, Dockerfiles, pruebas automatizadas ni deployment. El equipo debe definir juegos, tipos de eventos, métricas y comunicación entre módulos.

Consultar [arquitectura](docs/architecture.md) y [contrato de eventos](docs/events.md).

## Requisitos académicos principales

- Go controla la ingestión, validación y procesamiento concurrente.
- Scala implementa análisis con colecciones y operaciones funcionales.
- Ambos módulos son independientes y se comunican mediante un mecanismo documentado.
- Deben existir simulación realista, estadísticas por jugador y partida, análisis combinado y al menos una regla basada en secuencias.
- Se requieren Docker, CI/CD, ejecución integrada y una versión desplegada funcional.

## Desarrollo

Los comandos de instalación, pruebas y ejecución se documentarán después de elegir la estructura y herramientas de cada módulo. No se requiere instalar nada para leer esta documentación.
