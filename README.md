# GameStats

Plataforma académica multijuego del curso de Lenguajes de Programación. Go
recibe acontecimientos y coordina partidas; Scala calcula estadísticas y
patrones mediante perfiles declarativos y transformaciones funcionales.
El simulador es un productor HTTP independiente, no un videojuego gráfico.

## Instalar y evaluar sin herramientas de desarrollo

Descargar **GameStats-Entrega-v1.0.0.zip** desde la
[Release v1.0.0](https://github.com/JusMH/gamestats/releases/tag/v1.0.0),
extraerlo y abrir una terminal en su carpeta. No usar «Source code (zip)».
Requiere Docker Desktop o motor compatible, Compose y contenedores Linux/amd64;
la primera descarga de imágenes necesita Internet. No requiere Git, Go,
Java, Scala, sbt ni Python.

```sh
docker compose up -d --wait
docker compose ps
docker compose run --rm simulator --run-id evaluacion-01
docker compose run --rm simulator --mode negative --run-id negativos-01
docker compose down
```

Go está disponible en http://127.0.0.1:8080/health; Scala solo en la red interna.
Cada nueva demostración requiere otro run-id mientras siga vivo el proceso Go.
Reiniciar Go pierde todas las partidas: no existe persistencia histórica.
Los manuales incluidos explican instalación y evaluación. Ver
[distribución, digests y evidencias](docs/academic-distribution.md).

## Estado comprobado

- Ingestión v1, ciclo de vida, duplicados, concurrencia y snapshots Go implementados.
- Scala implementa count/sum/avg, duración, ventanas temporales y tablero de Gato.
  Los 13 fixtures completos se comparan con el motor puro y HTTP.
- El simulador produce seis partidas: victoria y empate de Gato, Snake con/sin
  ráfaga y dos carreras. Cada demo verifica 50 eventos y 60 consultas,
  48 abiertas y 12 cerradas, contra expectativas independientes.
- Docker Compose y CI están implementados. La
  [CI del commit de v1.0.0](https://github.com/JusMH/gamestats/actions/runs/39104918511)
  pasó, incluidos ambos detectores de carreras Go.
- Las tres imágenes GHCR y los tres adjuntos de v1.0.0 están publicados.
  La [evaluación anónima, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2)
  pasó, pero **el workflow completo terminó en failure por Finalize**.
  Se recuperó manualmente la Release con el mismo ZIP evaluado.
  No se presenta esa automatización como completamente aprobada.

La modalidad de deployment elegida es **local con Docker Compose**: Go y Scala
son servicios independientes en el equipo del evaluador. Publicar imágenes y
un ZIP no equivale a alojar el servicio. No hay deployment remoto ni nube.
La [auditoría final](docs/academic-audit.md) relaciona el enunciado original,
el código, las evidencias y las limitaciones, incluido el defecto de Finalize.

## Arquitectura y documentación

| Componente | Responsabilidad |
| --- | --- |
| [go-ingestion](go-ingestion/README.md) | HTTP, validación del sobre, estado en memoria, sincronización y puente a Scala. |
| [scala-analysis](scala-analysis/README.md) | Motor funcional sin estado acumulado, perfiles, métricas y analizadores. |
| [simulator](simulator/README.md) | Escenarios deterministas, concurrencia por partida y verificación HTTP. |
| [CI y publicación](.github/workflows/README.md) | Verificaciones automáticas y publicación manual controlada. |

El [índice documental](docs/README.md) clasifica todos los Markdown y separa
contratos, manuales y documentos históricos. Empezar por
[arquitectura](docs/architecture.md), [eventos v1](docs/events.md),
[integración HTTP](docs/integration-api.md) y
[perfiles componibles](docs/analysis-templates.md).
Para conectar otro productor, ver [integrar nuevos juegos](docs/integrating-new-games.md).

GET /results toma un snapshot completo y consulta Scala bajo demanda, tanto
con la partida abierta como cerrada. No hay push, caché, análisis automático
al cerrar ni contadores acumulados entre consultas. Los juegos sin perfil
siguen siendo aceptados por Go y obtienen arrays analíticos vacíos en Scala.

## Desarrollo y reproducción desde fuentes

Con el repositorio y Docker, desde la raíz:

```sh
docker compose up -d --build --wait
docker compose run --rm simulator --run-id desarrollo-01
docker compose down
```

El Compose raíz construye desde fuentes; el ZIP usa imágenes publicadas fijadas
por digest. Ver [guía Docker](docs/docker.md) para logs, salud y verificación.
Los README de cada componente explican ejecución sin contenedores y pruebas.
Ambos go.mod declaran Go 1.22; la versión efectivamente comprobada en CI/Docker
es Go 1.26.5. Scala usa 3.3.7, sbt 1.11.7 y JDK 21.

No se implementan bases de datos, procesamiento incremental, registro dinámico,
videojuegos completos, Prolog, WebSockets/SSE ni reglas universales.
Los próximos trabajos están en la [hoja de ruta](docs/implementation-roadmap.md).
