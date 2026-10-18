# GameStats CI — implementación 7

[ci.yml](ci.yml) es el workflow ejecutable. Se activa en Pull Requests dirigidos
a main, pushes a main y mediante workflow_dispatch. No filtra rutas: todos los
PR ejecutan las verificaciones completas, incluidos cambios documentales.
La ejecución manual aparece cuando el workflow ya existe en la rama por defecto.

## Trabajos y fallos

Todos utilizan runners oficiales ubuntu-24.04. Los trabajos de pruebas son
independientes; un fallo no cancela las comprobaciones de los otros módulos.

| Trabajo | Comprobaciones | Timeout |
| --- | --- | --- |
| Go / go-ingestion | build, test, vet y test -race | 15 min |
| Go / simulator | build, test, vet y test -race | 15 min |
| Scala / engine, HTTP and packaging | compile, MUnit, dockerStage y check-stage.py | 25 min |
| Contracts / fixtures, infrastructure and workflow | actionlint, Compose, fixtures y 16 pruebas unittest | 15 min |
| Docker / real multigame integration | docker/verify.py, evidencia y limpieza | 75 min |
| CI result | Resumen y comprobación de éxito de todos los trabajos anteriores | 5 min |

Go utiliza módulos separados, CGO_ENABLED=1 y CC=gcc. Antes de compilar se
comprueban GCC y la configuración CGO; el detector no se omite ni se convierte
en una advertencia. Una carrera detectada hace fallar el trabajo.

MUnit compara los 13 fixtures completos tanto en el motor puro como por HTTP.
check-stage.py arranca el JAR empaquetado sin sbt, vuelve a comparar los 13
fixtures y comprueba que perfiles vacíos impiden iniciar. No cambia el motor.

Infraestructura descubre y ejecuta la suite unittest existente con verbosity=2.
Exige exactamente 16 pruebas, incluso después de ejecutarlas: cero pruebas o
un cambio no revisado de su cantidad falla. Cuando se amplíe esa suite, revisar
explícitamente este guard. El comando equivalente habitual sigue siendo:
python -m unittest discover -s docker -p "test_*.py" -v.

CI result falla si cualquier dependencia termina failed, cancelled o skipped.
No hay continue-on-error ni reintentos indiscriminados. La cola por PR/referencia
no cancela una integración anterior mientras limpia sus recursos.

## Versiones y procedencia

| Herramienta | Versión |
| --- | --- |
| Runner | ubuntu-24.04 |
| Go | 1.26.5, GOTOOLCHAIN=local |
| JDK | Eclipse Temurin 21; setup-java resuelve la actualización disponible, sin fijar patch/build |
| Scala / sbt | 3.3.7 / 1.11.7, definidos por el módulo |
| Python | 3.12, resuelto por setup-python |
| actionlint | 1.7.12, go run con versión fija y verificación normal de módulos Go |
| checkout / setup-go | 6.1.0 / 6.5.0 |
| setup-java / setup-python | 5.7.0 / 6.3.0 |
| upload-artifact | 6.0.0 |
| Docker / Compose | Versiones del runner; se registran y Compose debe ser >= 2.20 |

Las acciones oficiales están fijadas a SHA completo, con su versión anotada;
los tags y action.yml de esos commits se consultaron en sus repositorios
oficiales. No se afirma que sean las versiones más recientes.

sbt se prepara directamente desde Maven Central, sin depender del wrapper
PowerShell ni de una instalación implícita del runner. Se descarga
sbt-launch-1.11.7.jar y se exige SHA-256
f92a2095ac75008764fe3b2b793ffe624c4fbef5bfd9b0022e4bc2daf668c651,
igual al wrapper Windows y al Dockerfile aprobados. project/repositories
limita la resolución al repositorio local y Maven Central.

setup-java reutiliza cachés sbt/Ivy/Coursier, cuya clave incluye build.sbt y
los archivos project relevantes. La ausencia de caché no evita las pruebas.
Go no activa su caché de acción porque ambos módulos son stdlib y no tienen
go.sum. Docker construye una sola vez las tres imágenes dentro de verify.py;
no se agrega otro build duplicado en el mismo trabajo.

## Integración real y artefactos

El proyecto y run-id son ci-GITHUB_RUN_ID-GITHUB_RUN_ATTEMPT. Solo se utilizan
IDs numéricos de GitHub: cumplen las restricciones de longitud y caracteres
del verificador, sin interpolar títulos o nombres de ramas en shell.
La evidencia se escribe en docker/evidence/ci/, inicialmente vacío en el runner.

El workflow reutiliza docker/verify.py sin copiar su secuencia al YAML:
construye las tres imágenes, espera healthchecks y ejecuta el simulador real
contra Go y Scala. Comprueba dos demos de seis partidas, 50 eventos y 60
consultas cada una, 48 abiertas y 12 cerradas por demo, repetición determinista,
tres negativos, seis rechazos 409, otro run-id y perfiles Scala inexistentes.
El criterio de concurrencia sigue siendo pico >= 2, sin exigir exactamente 6.

El verificador limpia su proyecto en finally y comprueba etiquetas antes de
eliminar tareas. El workflow no ejecuta prune ni borra recursos de otros
proyectos. El timeout del paso de integración es 65 minutos, dejando margen
para diagnósticos y subida de artefactos dentro de los 75 minutos del trabajo.
Una cancelación forzada o pérdida del runner puede interrumpir finally; en ese
caso no se declara limpieza confirmada y GitHub descarta la VM efímera.

En Actions → GameStats CI → ejecución se encuentran logs, Job summary y:

- docker-evidence-RUN_ID-RUN_ATTEMPT: summary.json, construcción, inspect/salud,
  logs de servicios, demo.json, negative.json, duplicate.json, new-id.json,
  comandos y evidencia de limpieza.
- scala-tests-RUN_ID-RUN_ATTEMPT: reportes XML MUnit.

Ambos uploads usan always(), retención de 14 días y if-no-files-found: error.
Se conservan diagnósticos disponibles aunque el verificador falle. Una ejecución
aprobada exige summary.json con status=passed, los archivos esenciales y
resourcesReleased=true. Su ausencia no puede producir éxito silencioso.
Si checkout/setup falla antes de crear evidencia, el trabajo falla y el upload
informa que no encontró archivos; los logs del paso explican el error inicial.

## Seguridad

GITHUB_TOKEN solo tiene contents: read. checkout no conserva credenciales.
Se utiliza pull_request, nunca pull_request_target; los PR de forks no necesitan
secretos. No se publican imágenes, no se autentica contra registros ni se
modifican permisos o protección de main. Los artefactos contienen únicamente
reportes de escenarios deterministas y diagnósticos de contenedores sin secretos.
No añadir credenciales a variables Compose o logs sin revisar esta política.

## Reproducción local

Desde la raíz, con los runtimes indicados y GCC/CGO en Linux:

```sh
for module in go-ingestion simulator; do
  (cd "$module" && go build ./... && go test ./... && go vet ./... && CGO_ENABLED=1 CC=gcc go test -race ./...)
done
python docs/fixtures/analysis-v1/check.py
python -m unittest discover -s docker -p "test_*.py" -v
docker compose config --quiet
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 -color
python docker/verify.py --project ci-local-01 --run-id ci-local-01 --evidence docker/evidence/ci-local-01
```

Usar un proyecto/directorio de evidencia nuevo por ejecución. El procedimiento
Docker requiere Docker Linux; no requiere runtimes Go/Scala en el anfitrión.
Para Scala en Windows:

```powershell
cd scala-analysis
.\sbtw.ps1 compile test dockerStage
cd ..
python docker/check-stage.py
```

En Linux puede reproducirse exactamente la preparación del lanzador y la
invocación java del trabajo Scala; alternativamente, con sbt 1.11.7 instalado:

```sh
(cd scala-analysis && sbt compile test dockerStage)
python docker/check-stage.py
```

## Verificación realizada y límites

En esta entrega pasaron localmente actionlint 1.7.12 (sintaxis YAML, estructura,
expresiones e inputs), sintaxis de los cuatro scripts Python inline, guard real
de 16 pruebas, rechazo de cero pruebas y rechazo de evidencia ausente.
No había ShellCheck ni Pyflakes locales: actionlint local desactivó solo esos
linters opcionales; en CI utiliza su comportamiento normal y ShellCheck si está
disponible en el runner. Los SHA se comprobaron contra tags y action.yml oficiales.

También pasaron compilación, pruebas, vet y -race de ambos módulos en un
contenedor Linux/amd64 con Go 1.26.5, CGO=1 y GCC Alpine 15.2.0. Esta verificación
resuelve la limitación del host Windows sin instalar compiladores globales;
no equivale a haber ejecutado el runner Ubuntu de GitHub.

Scala: 143 pruebas aprobadas, compile/dockerStage correctos; check-stage verificó
13 fixtures y rechazo de perfiles vacíos. Infraestructura: 16 pruebas aprobadas;
contratos: 30 JSON, 13 pares y 9 mutaciones inválidas. Integración real local:
proyecto ci-local-20261017-7, dos demos completas, negativos/duplicados/perfiles
ausentes correctos y resourcesReleased=true. Evidencia local ignorada en
docker/evidence/ci-local-20261017-7/summary.json (archivo local ignorado por Git).

CI fue aprobada y fusionada mediante el PR #11, después de corregir la selección
de Temurin a Java 21. Las comprobaciones locales anteriores se conservan como
registro; ya no representan una primera ejecución remota pendiente.
CI result sigue siendo la propuesta de check obligatorio, sin cambiar reglas
del repositorio. ci.yml mantiene contenidos y permisos de solo lectura.

## Publicación académica independiente — Etapa 8

[academic-release.yml](academic-release.yml) se inicia manualmente mediante
workflow_dispatch desde main, con versión, SHA completo revisado y confirmación
`PUBLICAR v1.0.0`. Exige CI exacta antes de publicar imágenes GHCR. No se crea
previamente un tag o Release. packages: write solo corresponde a images;
contents: write solo a finalize. CI permanece sin cambios y con permisos de lectura.

La secuencia diseñada es evaluación anónima → borrador → adjuntos verificados
→ publicación final, compatible con Releases inmutables. **La ejecución real
no completó Finalize**; no debe confundirse el diseño o sus pruebas locales con
un éxito de ese job remoto.

La [guía técnica](../../docs/academic-distribution.md) registra los digests,
permisos y recuperación. En el [run 39105352591, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2),
approval/images/evaluation pasaron y finalize falló con
`Draft creation not confirmed; retry failed job`. Se publicó v1.0.0 mediante
recuperación manual, con el mismo ZIP que pasó evaluación. El workflow completo
terminó failure. El [run CI 39104918511](https://github.com/JusMH/gamestats/actions/runs/39104918511)
del commit publicado sí terminó success; son workflows distintos.

Tras hacer públicos los paquetes, Re-run failed jobs reutilizó el artefacto
release-images-39105352591-1 y sus digests, sin reconstrucción ni push. No usar
Re-run all jobs para sobrescribir una versión. La Release manual carece del
marcador lógico esperado por Finalize; no reintentarlo ciegamente sobre v1.0.0.

Las 24 pruebas locales de distribución complementan las 16 de infraestructura,
pero no certificaron la creación real del borrador. Es necesario reparar y
volver a comprobar Finalize en un cambio separado antes de otra publicación.
La distribución ya permite deployment **local** con Compose; no hay alojamiento remoto.

## Referencias oficiales

- [Eventos y sintaxis de workflows](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions).
- [checkout](https://github.com/actions/checkout), [setup-go](https://github.com/actions/setup-go),
  [setup-java](https://github.com/actions/setup-java), [setup-python](https://github.com/actions/setup-python).
- [Artefactos oficiales](https://github.com/actions/upload-artifact).
- [actionlint](https://github.com/rhysd/actionlint).
