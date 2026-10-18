# GameStats con Docker Compose — implementación 6

Tres imágenes independientes: Go para ingestión, Scala para análisis y un
simulador finito bajo demanda. Los servicios mantienen los contratos existentes.
Scala no publica un puerto al anfitrión. No hay persistencia ni proxy.
Esta guía opera el Compose de desarrollo; la [distribución publicada](academic-distribution.md)
permite instalar sin fuentes. Los resultados de las secciones Etapa 6
son históricos; CI y publicación se añadieron después.

## Requisitos

- Docker Engine con contenedores Linux y Docker Compose v2.20+.
  En Windows/macOS, Docker Desktop debe tener el motor iniciado.
- Acceso a Docker Hub y Maven Central durante la primera construcción.
- Puerto local 8080 libre, o configurar GO_PORT.
- No se necesitan Go, Scala, sbt ni JDK en el anfitrión para construir/ejecutar.
- Python 3.10+ es opcional: solo para los verificadores automatizados.

Usar los comandos desde la raíz del repositorio. No cambiar el contexto Docker
ni ejecutar como administrador para estas instrucciones.

## Windows y rutas con tildes

Los pasos consolidados para CMD están en [Windows CMD](windows-cmd.md).
Para construir desde fuentes, usar una ruta sin tildes en todos sus niveles.
En el equipo revisado, el contexto bajo `Programación/gamestats-revisión`
falló antes de compilar con `x-docker-expose-session-sharedkey` y
`non-printable ASCII characters`. La misma copia sin tildes construyó y
pasó las pruebas integradas. El paquete publicado usa imágenes por digest
y no realiza esa construcción de fuentes.

## Arranque y demostración

~~~sh
docker compose config --quiet
docker compose up -d --build
docker compose ps
docker compose run --rm simulator --run-id docker-demo-01
~~~

El arranque normal deja activos solamente Go y Scala. simulator tiene el perfil
tools; invocarlo explícitamente con run lo habilita para esa tarea y termina
con el código de salida del programa. No usar up con el perfil tools para una
tarea que se pretende ejecutar bajo demanda.

El simulador verifica por defecto sus seis escenarios propios: victoria/empate
de Gato, Snake con/sin ráfaga y dos carreras. Consulta cinco estados por partida,
dos veces cada uno, comparando todas sus expectativas. Un HTTP 200 por sí solo
no implica éxito. Una discrepancia produce salida 1.

Con ambos servidores healthy:

~~~sh
docker compose run --rm simulator --mode negative --run-id docker-negative-01
docker compose run --rm simulator --run-id docker-demo-02
~~~

Cada ejecución normal utiliza otro run-id. Repetir docker-demo-01 en el mismo
proceso Go debe terminar con salida 1 y errores HTTP 409 por eventId duplicado.
No considerar cualquier salida 1 como prueba aprobada: se debe verificar el
motivo. El procedimiento automatizado comprueba los seis mensajes esperados.

## Red, puertos y configuración

| Componente | Configuración en Compose |
| --- | --- |
| Go | SCALA_SERVICE_URL=http://scala-analysis:9000; escucha :8080. |
| Scala | SCALA_HOST=0.0.0.0, SCALA_PORT=9000, SCALA_PROFILES_DIR=/app/profiles. |
| Simulador | --go-url http://go-ingestion:8080; verificación habilitada. |
| Red | Red bridge predeterminada del proyecto Compose y DNS de servicios. |
| Acceso del anfitrión | Go en 127.0.0.1:8080; Scala sin puerto publicado. |

localhost solo aparece en los healthchecks del propio contenedor o en
consultas del anfitrión. Entre componentes se usan los nombres DNS anteriores.

Opciones Compose, configurables mediante variables de entorno o .env local:

| Variable | Valor por defecto |
| --- | --- |
| GO_BIND_ADDRESS | 127.0.0.1 |
| GO_PORT | 8080 |
| SIM_RUN_ID | docker-demo-01 |
| SIM_MODE | demo |
| SIM_CONCURRENCY | 6 |
| SIM_TIMEOUT | 10s |
| SIM_MAX_DURATION | 2m |

Un .env local no se versiona ni entra al contexto de construcción.
Para otro puerto, en CMD:

~~~cmd
set "GO_PORT=8081"
docker compose up -d --build
~~~

En una shell POSIX:

~~~sh
GO_PORT=8081 docker compose up -d --build
~~~

Las variables SIM_* afectan el command predeterminado, usado con
docker compose run --rm simulator sin argumentos. Si se proporcionan argumentos,
Compose reemplaza command y se aplican los defaults originales de la CLI para
los flags omitidos. La URL interna permanece en entrypoint, por lo que:

~~~sh
docker compose run --rm simulator --run-id defensa-01 --concurrency 3 --timeout 15s --max-duration 3m
~~~

sigue apuntando a Go por DNS. También se conserva --go-url explícito para
utilizar otro destino; el último flag prevalece según el parser Go existente.
No se reescribió el simulador para agregar estas opciones.

## Imágenes y construcción

Todas las bases son imágenes oficiales, con etiqueta exacta y digest del
índice multiarquitectura; no se utiliza latest:

| Uso | Base |
| --- | --- |
| Compilación Go/simulador | golang:1.26.5-alpine3.23 |
| Runtime Go/simulador | alpine:3.23.4 |
| Compilación Scala | eclipse-temurin:21.0.12_8-jdk-alpine-3.23 |
| Runtime Scala | eclipse-temurin:21.0.12_8-jre-alpine-3.23 |

Los digests están fijados en cada Dockerfile y se consultaron en el registro
oficial. Actualizarlos debe ser una acción explícita con pruebas; no se
promete reproducibilidad binaria byte por byte de los JAR generados.

Go y simulador se compilan con CGO_ENABLED=0, GOTOOLCHAIN=local y -trimpath.
Solo se copian los ejecutables y certificados CA a Alpine. No hay compilador
Go ni fuentes en las imágenes finales.

Scala conserva Scala 3.3.7, sbt 1.11.7 y las dependencias ya aprobadas.
El builder descarga el lanzador sbt de Maven Central y verifica el mismo
SHA-256 que sbtw.ps1. El nuevo task dockerStage crea app.jar y lib/ con las
cinco dependencias externas de runtime, excluyendo MUnit. No agrega plugins
externos ni modifica la lógica del servicio.

El runtime contiene JRE 21, app.jar, lib/ y los tres perfiles JSON aprobados
copiados a /app/profiles. Ejecuta directamente:

~~~text
java -XX:MaxRAMPercentage=70.0 -cp /app/app.jar:/app/lib/* gamestats.Main
~~~

No necesita sbt, sus cachés ni las fuentes. Perfiles ausentes, vacíos o inválidos
impiden arrancar mediante la validación existente, con error visible y salida 1.
Los perfiles se incluyen en la imagen; un cambio exige reconstruirla.

Los tres Dockerfiles usan el contexto raíz. .dockerignore deniega por defecto
todo y permite únicamente los módulos necesarios y perfiles; además excluye
cachés, target, ejecutables, evidencia, .env, claves, credenciales e IDEs.
No se envían fixtures ni configuraciones locales al builder.

## Privilegios, salud y readiness

Todos los runtimes usan UID/GID 10001:10001, sin capacidades Linux y con
no-new-privileges. Go y Scala tienen filesystem de solo lectura; Scala dispone
de /tmp temporal para la JVM. El simulador conserva /reports escribible para
evidencia opcional dentro de su contenedor, sin volumen persistente.

Go y Scala ejecutan wget contra GET /health. wget proviene de BusyBox/Alpine;
cada Dockerfile comprueba command -v wget durante la construcción.
Los healthchecks tienen timeout, intervalo, start-period y reintentos.

Go depende de Scala healthy. El simulador depende de ambos healthy.
Esto ordena el arranque; no es un supervisor que reinicie dependientes si un
servicio falla después. Un /health de Go correcto no prueba su puente: la
simulación verificada mediante GET /results es la comprobación analítica real.

## Logs, salud y apagado

~~~sh
docker compose logs --tail 100 go-ingestion scala-analysis
docker compose logs -f
docker compose ps
docker compose exec -T go-ingestion wget -q -O - http://127.0.0.1:8080/health
docker compose exec -T scala-analysis wget -q -O - http://127.0.0.1:9000/health
docker compose exec -T go-ingestion wget -q -O - http://scala-analysis:9000/health
docker compose down
~~~

down elimina los contenedores/red del proyecto; no realiza prune global.
Las imágenes/cachés se conservan normalmente. Reconstrucción explícita:

~~~sh
docker compose --profile tools build
docker compose up -d
# Para reconstruir sin caché:
docker compose --profile tools build --no-cache
~~~

Go guarda todo en memoria. Detener, reiniciar o recrear su proceso pierde
todas las partidas, incluidas las cerradas. No hay volumen de datos,
recuperación ni resultados históricos persistidos.

## Verificación reproducible de extremo a extremo

Con Python 3.10+ y el motor Docker disponible, desde la raíz:

~~~sh
python docker/verify.py
~~~

Funciona también en Windows: py -3 docker/verify.py o python docker/verify.py.
No necesita herramientas Go/Scala locales.

El procedimiento usa un nombre de proyecto nuevo y puerto Go asignado por
Docker en loopback. Rechaza un proyecto que ya tenga contenedores, redes o imágenes con ese prefijo.
Realiza:

1. Configuración Compose y disponibilidad del motor Linux.
2. Construcción de las tres imágenes.
3. Arranque con --wait y comprobación healthy de ambos servicios.
4. Inspección del usuario y de las utilidades de runtime; rechazo de perfiles ausentes con salida 1 y diagnóstico esperado.
5. Simulador normal: seis partidas, 50 eventos y 60 consultas verificadas.
6. Tres casos negativos: sobre sin mutación, dominio inválido y juego sin perfil.
7. Repetición del ID: salida 1 y seis errores HTTP 409 de duplicado.
8. Nueva ejecución con otro ID: otras seis partidas y 60 consultas verificadas.
9. Logs y limpieza de sus contenedores, imágenes locales y red.

Los dos runs normales deben sumar 100 eventos y 120 respuestas analíticas;
los casos negativos son comprobaciones adicionales, separadas.
Cada run comprueba 48 respuestas abiertas, 12 cerradas y solicitudes solapadas.

Evidencia opcional mediante --evidence RUTA, por defecto
docker/evidence/<proyecto>/, ignorada por Git. Incluye construcción,
configuración, inspect, logs, demo.json, negative.json, duplicate.json,
new-id.json y summary.json.

El script conserva las tareas detenidas solo el tiempo necesario para copiar
su reporte, después las elimina. Comprueba etiquetas antes de borrar un
contenedor nombrado. No monta rutas Windows ni usa volúmenes persistentes.
No elimina recursos de otros proyectos ni ejecuta system prune.

La evidencia debe escribirse en una carpeta vacía para no sobrescribir una
ejecución anterior. Con --project se puede identificar una prueba, pero no
reutilizar recursos existentes. Un fallo de motor/build/HTTP/resultado o de
limpieza produce salida distinta de cero; nunca se declara aprobado solo
porque se hayan generado los archivos.

## Comprobaciones de desarrollo

~~~sh
docker compose config --quiet
python -m unittest discover -s docker -p "test_*.py" -v
~~~

Las pruebas de infraestructura validan configuración y controles del verificador;
no sustituyen la integración con contenedores reales.

Para comprobar por separado el empaquetado Scala con JDK local:

~~~powershell
cd scala-analysis
.\sbtw.ps1 dockerStage compile test
cd ..
python docker/check-stage.py
~~~

En POSIX se puede usar sbt dockerStage compile test. check-stage inicia únicamente
el JAR empaquetado y sus dependencias, verifica /health y los 13 fixtures
completos, y comprueba el fallo ante perfiles vacíos. No se presenta como una
prueba de la imagen Docker.

## Resultado de esta entrega

Verificado con Docker Engine 29.7.2 (Linux/amd64), cliente Windows/amd64
29.7.2 y Docker Compose 5.3.1. El motor inicialmente no respondía; tras iniciar
Docker Desktop instalado quedó disponible, sin reinstalaciones, cambios de
configuración global ni privilegios de administrador.

| Comprobación ejecutada | Resultado |
| --- | --- |
| docker compose config | Correcto, incluyendo perfil tools. |
| Pruebas de infraestructura/verificador | 16 aprobadas. |
| Construcción de las tres imágenes | Correcta, con bases/versiones/digests fijados. |
| Healthchecks y usuario de runtime | Ambos healthy; UID/GID 10001:10001; wget disponible. |
| Puente analítico real | Simulador → Go → Scala, sin mocks. |
| Demo normal | 6 partidas, 50 eventos y 60 consultas verificadas. |
| Consultas abiertas/cerradas | 48 abiertas y 12 cerradas por demo, pico de 6 solicitudes simultáneas. |
| Negativos de sobre/dominio/juego desconocido | Tres comprobaciones aprobadas. |
| Reutilización del ID | Salida 1 por seis duplicados HTTP 409; motivos comprobados. |
| Nueva ejecución con otro ID | Otras 6 partidas, 50 eventos y 60 consultas verificadas. |
| Perfiles ausentes en imagen Scala | Salida 1: Cannot start analysis service: profile_directory at profiles. |
| Limpieza | Contenedores, red e imágenes locales del proyecto eliminados; sin tocar proyectos ajenos. |
| Comandos manuales up/run | Probados con otro proyecto: construcción automática del simulador y readiness correctos. |
| Go ingestión y simulador | go test ./... y go vet ./... correctos en ambos módulos. |
| Scala | dockerStage, compilación y 143 pruebas aprobadas. |
| Artefacto Scala sin sbt en runtime | Health y 13 respuestas completas de fixtures; perfiles vacíos rechazados. |
| Verificador documental | 30 JSON, 13 pares y 9 mutaciones inválidas. |

La evidencia final local está en docker/evidence/final/summary.json,
demo.json, negative.json, duplicate.json y new-id.json, junto con inspect,
construcción, logs y limpieza. status=passed, containerIntegrationExecuted=true,
invalidProfilesRejected=true y resourcesReleased=true. No se versiona evidencia.
El ensayo adicional de comandos manuales se guardó en docker/evidence/manual/.

No se modificaron contratos, perfiles, fixtures ni implementaciones de dominio.
Las comprobaciones de empaquetado local y de contenedores se distinguen:
ambas se ejecutaron, pero solo el procedimiento Compose acredita la integración.

Limitaciones: validación de contenedores realizada en Linux/amd64; no se ejecutó
la variante ARM64. La primera construcción descarga bases y dependencias y puede
tardar varios minutos con una conexión lenta. Las ejecuciones posteriores usan
caché. Los datos de Go siguen siendo volátiles. No hay autenticación, retención,
reintentos idempotentes. La publicación GHCR, CI y el despliegue local Compose
se documentan en [la auditoría actual](academic-audit.md); no hay alojamiento remoto.
Las cachés de BuildKit y bases compartidas se conservan: no se hizo prune global.


## Auditoría de recuperación (2026-10-17)

Se recuperó la implementación existente en infra/docker-compose, con base
166621632c6c, sin reconstruir imágenes ni repetir la integración completa.
Los reportes demo/new-id, negativos, duplicados, inspect, build y limpieza
se contrastaron entre sí; los logs manuales también conservan seis partidas
verificadas. Los XML Scala registran 143 pruebas, cero fallos y cero errores.
Estos resultados son evidencia recuperada, no ejecuciones nuevas de Scala.

En la recuperación se ejecutaron nuevamente compose config --quiet,
las 16 pruebas unittest de infraestructura (descubrimiento confirmado),
validación sintáctica de los tres scripts Python, go test ./... y go vet ./...
en ambos módulos Go y git diff --check. Las pruebas Go usaron su caché válida.
Docker Engine respondió como Linux/amd64 y el proyecto de verificación final
no conserva contenedores ni red. No se necesitaron descargas nuevas.

Se corrigieron frases antiguas de los README de Go y simulador sobre Scala
pendiente y Docker inexistente. No se detectaron archivos truncados,
marcadores provisionales ni cambios en contratos, perfiles, fixtures o lógica
de dominio. La evidencia y los cachés Python continúan ignorados por Git.

## Archivos de esta etapa

Creados:

| Archivo | Responsabilidad |
| --- | --- |
| [compose.yaml](../compose.yaml) | Red, variables, salud, seguridad y tarea bajo demanda. |
| [.dockerignore](../.dockerignore) | Contexto raíz restringido y exclusión de secretos/artefactos. |
| [Dockerfile Go](../go-ingestion/Dockerfile) | Compilación Go y runtime Alpine. |
| [Dockerfile Scala](../scala-analysis/Dockerfile) | Construcción JDK/sbt y runtime JRE sin sbt. |
| [DockerPackaging.scala](../scala-analysis/project/DockerPackaging.scala) | Task dockerStage, sin plugins externos. |
| [Dockerfile simulador](../simulator/Dockerfile) | Cliente finito con flags originales. |
| [verify.py](../docker/verify.py) | Integración real y limpieza de proyecto exclusivo. |
| [test_verify.py](../docker/test_verify.py) | Configuración, motivos de fallos y protección de recursos ajenos. |
| [check-stage.py](../docker/check-stage.py) | Verificación opcional del artefacto con JDK local. |
| [docker.md](docker.md) | Guía técnica y evidencia de esta etapa. |

Modificados: [.gitignore](../.gitignore), [README principal](../README.md),
[README Go](../go-ingestion/README.md), [README Scala](../scala-analysis/README.md),
[README simulador](../simulator/README.md) y [hoja de ruta](implementation-roadmap.md).
No se añadieron workflows, publicaciones ni infraestructura de deployment.

## Referencias

- [Servicios, command, entrypoint, healthchecks y dependencias Compose](https://docs.docker.com/reference/compose-file/services/).
- [Ejecución de tareas con Compose run](https://docs.docker.com/reference/cli/docker/compose/run/).
- [Perfiles Compose](https://docs.docker.com/compose/how-tos/profiles/).
- [Imagen oficial Eclipse Temurin](https://hub.docker.com/_/eclipse-temurin).
- [Imagen oficial Go](https://hub.docker.com/_/golang).
- [Imagen oficial Alpine](https://hub.docker.com/_/alpine).

## Continuidad histórica

Al cerrar la Etapa 6 se propuso CI como siguiente tarea. CI y la distribución
v1.0.1 ya están verificadas, incluida la reparación de Finalize. Los cambios
posteriores de avanceFran se prueban desde fuentes; ver
[hoja de ruta](implementation-roadmap.md) y [verificación actual](verificacion-entrega.md).
