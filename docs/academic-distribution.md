# Publicación GHCR y distribución académica — Etapa 8

## Estado y alcance

CI del PR #11 está aprobada y fusionada. La Etapa 8 prepara publicación y
entrega local con Docker Desktop; **no se publicaron imágenes, tags, Releases
ni un ZIP definitivo durante esta implementación**. El ZIP de prueba local
usa digests ficticios y está marcado PRUEBA / NO ENTREGAR.

No se ofrece un servicio remoto alojado. El profesor recibirá configuración y
manuales, descargará imágenes la primera vez y ejecutará Go y Scala en su PC.
No necesita herramientas de desarrollo ni Git. La distribución solo estará
lista tras superar la verificación anónima de las tres imágenes y del ZIP.

## Rutas, versión y artefactos

Versión inicial prevista: v1.0.0; versiones estables vMAJOR.MINOR.PATCH.

| Componente | Paquete previsto |
| --- | --- |
| Go | ghcr.io/jusmh/gamestats-go-ingestion:v1.0.0 |
| Scala | ghcr.io/jusmh/gamestats-scala-analysis:v1.0.0 |
| Simulador | ghcr.io/jusmh/gamestats-simulator:v1.0.0 |

No hay latest como referencia de entrega. Las tres imágenes usan la misma
versión. Sus digests efectivos se obtienen de RepoDigests después de docker push
y se guardan en images.json con commit, CI aprobada, repositorio y plataforma.
El ZIP utiliza image: ghcr.io/jusmh/PAQUETE:VERSION@sha256:DIGEST_REAL.
El tag facilita lectura; el digest fija el contenido aunque un administrador
mueva posteriormente el tag. No se inventan digests en una distribución real.

Se reutilizan los Dockerfiles aprobados sin cambios, Linux/amd64 únicamente:
Go 1.26.5/Alpine 3.23.4 y Scala con builder JDK/runtime JRE 21, Scala 3.3.7 y
sbt 1.11.7. Se conservan usuarios 10001, salud HTTP y perfiles en la imagen.
OCI source, revision, version y title se agregan durante la construcción,
relacionando los paquetes con https://github.com/JusMH/gamestats.

## Compatibilidad con Releases inmutables

El flujo anterior release: published → adjuntar al final era incompatible con
Releases inmutables: después de publicar no se pueden agregar o cambiar
adjuntos. GitHub recomienda borrador → adjuntos completos → publicación.
No se consulta ni modifica la configuración de inmutabilidad del repositorio.
El nuevo flujo funciona también cuando esa opción esté habilitada.

## Workflow, aprobación humana y controles

[academic-release.yml](../.github/workflows/academic-release.yml) es independiente
de ci.yml, que no cambia. Solo se activa mediante workflow_dispatch desde main.
El mantenedor debe revisar el SHA completo de main e introducir tres campos:
version (por ejemplo v1.0.0), reviewed_commit (SHA de 40 caracteres) y confirmation
exactamente `PUBLICAR v1.0.0`. El SHA revisado debe coincidir con GITHUB_SHA del
dispatch; si main cambió antes de iniciar, hay que revisar el nuevo commit.
La acción manual autoriza publicar esa versión y ese commit. No se requieren
nuevos environments ni cambios de permisos globales para esa aprobación.

Secuencia: aprobación manual → CI exacta → imágenes GHCR/digests → ZIP candidato
→ descarga e integración anónimas del ZIP → borrador con todos los adjuntos
verificados → publicación definitiva. El ZIP candidato es interno; nunca se
adjunta a una Release pública antes de superar la instalación completa.

| Trabajo | Permisos del token | Responsabilidad |
| --- | --- | --- |
| approval | contents: read, actions: read | Contexto manual/main, versión/SHA/confirmación, ausencia de tag/Release y CI exacta. |
| images | contents: read, packages: write | Construir tres imágenes, rechazar tags existentes, publicar y registrar digests. |
| evaluation | contents: read, actions: read | Generar ZIP candidato y verificar su instalación anónima. |
| finalize | contents: write, actions: read | Crear borrador al final, cargar y verificar bytes de los adjuntos y publicar. |

El commit debe pertenecer a main. Se consulta la API de Actions por ci.yml,
head_sha exacto, evento push, rama main, status completed y conclusion success;
además se exigen los seis jobs esperados, todos exitosos. No basta otro PR verde.
Si CI sigue pendiente, el flujo falla antes de cualquier publicación.

No se crea previamente ningún tag o Release. approval e images rechazan un tag
o Release ya existente para esa versión; solo finalize está autorizado a
crearlos después de la evaluación. La Release apunta al SHA aprobado, no a un
main que podría haber avanzado. En la finalización, --target utiliza ese SHA.

Los artefactos incluyen run-id e intento. Los consumidores toman el nombre del
output del productor exitoso, no lo recalculan con el intento actual. Re-run
failed jobs tras cambiar los paquetes a públicos reutiliza images.json del job
images exitoso; no ejecuta docker build ni push. No usar Re-run all jobs ni un
nuevo dispatch para intentar reutilizar esa versión: se rechazarán tags existentes.

Las publicaciones se serializan. Antes de construir y de publicar se consultan
todas las versiones de los paquetes; si el tag existe en alguno, se detiene sin
sobrescribir. Errores de permisos/servidor fallan. Un paquete inexistente devuelve
404. Estos controles no convierten los tags GHCR en inmutables frente a otros
administradores: la distribución se fija por digest para conservar su contenido.

Se construyen las tres imágenes antes del primer push. No hay transacción
atómica entre paquetes: ante un push parcial, el manifiesto guarda los digests
obtenidos y nunca se empaqueta uno incompleto. No se borran paquetes ni se
cambia su visibilidad automáticamente.

Se usa GITHUB_TOKEN, sin PAT; login por stdin y logout. El token no se incorpora
a builds ni artefactos. Las acciones están fijadas a SHA comprobados, incluido
download-artifact 7.0.0. No hay pull_request_target ni publicación desde un PR.

finalize comprueba versión, SHA, CI, evaluación anónima, limpieza, checksum del
ZIP y coincidencia del manifiesto embebido con images.json. Crea un draft propio
identificado por run-id y SHA, sube los tres adjuntos sin --clobber, los descarga
y compara byte a byte por SHA-256. Solo entonces publica con --draft=false.
El tag se crea al finalizar, apuntando al commit revisado. No se añade ningún
adjunto después de publicar. release-finalization.json registra URL, ID, SHA,
verificación de adjuntos y el estado immutable informado por GitHub.

## ZIP y Compose de evaluación

[distribution/compose.yaml](../distribution/compose.yaml) es una plantilla válida
con referencias v1.0.0 previstas, **todavía no publicadas**. No incluye build,
fuentes, mounts, rutas Windows ni Scala expuesto. Go solo publica loopback;
GO_PORT permite cambiar 8080. El simulador permanece en el perfil tools y
mantiene sus flags. Los healthchecks vienen de las imágenes aprobadas y las
dependencias usan service_healthy; Go se conecta a Scala por DNS interno.

release.py pack sustituye las referencias de esa plantilla por los digests del
manifiesto efectivo y crea:

```text
GameStats-Entrega-v1.0.0.zip
  GameStats-Entrega-v1.0.0/
    compose.yaml
    INSTALACION.md
    GUIA-DE-EVALUACION.md
    IMAGENES.json
SHA256SUMS.txt
```

No incluye fuentes, Python, Java, compiladores, cachés ni secretos. Los scripts
Python son herramientas del mantenedor/Actions, no requisitos del profesor.
El generador fija orden, timestamp y modo de archivos para que idénticas
entradas produzcan el mismo ZIP dentro del entorno de empaquetado. No promete
reproducibilidad binaria de las imágenes ni igualdad entre versiones de zlib.
Rechaza archivos de salida existentes. La Release recibe ZIP, SHA256SUMS.txt e
images.json; gh release upload no utiliza --clobber.

## Primera publicación después de revisar y fusionar el PR

1. Revisar/fusionar los cambios a main por decisión humana y esperar CI completa
   exitosa de ese commit (push/main). Copiar su SHA completo y revisar el contenido.
2. No crear tag ni Release previamente. En Actions → Academic distribution →
   Run workflow, seleccionar main, version=v1.0.0, reviewed_commit=SHA revisado
   y confirmation=`PUBLICAR v1.0.0`. Esta es la aprobación humana de publicación.
3. approval e images deben pasar. Revisar release-images-RUN_ID-RUN_ATTEMPT.
4. Si GHCR crea paquetes privados, un administrador debe autorizar Public para
   los tres en perfil JusMH → Packages → paquete → Package settings → Change
   visibility. Ser público el repositorio no garantiza descarga pública.
5. Después, usar **Re-run failed jobs**, dentro de la misma ejecución. Los jobs
   images/approval exitosos no se reconstruyen y se reutilizan sus digests.
6. evaluation debe acreditar status=passed, anonymousPulls=true y
   resourcesReleased=true. finalize crea entonces un borrador, carga todos los
   adjuntos, verifica su contenido y publica. No publicar ese borrador manualmente
   mientras el workflow trabaja: podría dejar una Release inmutable incompleta.
7. Descargar GameStats-Entrega-v1.0.0.zip y SHA256SUMS.txt desde la Release final,
   sin confundirlo con Source code (zip). No distribuir un ZIP de un job fallido.

La comprobación anónima utiliza DOCKER_CONFIG vacío temporal y compose pull
--policy always. No basta una descarga autenticada ni la existencia de capas
en caché. Después se verifica la demostración real desde el ZIP extraído.

## Recuperación ante fallos

- **CI no aprobada:** resolver el fallo, esperar CI exacta exitosa y reejecutar
  trabajos fallidos; no se publica antes de superar approval.
- **Build fallido antes del push:** corregir mediante otra versión aprobada o
  reintentar si la causa fue infraestructura y ningún tag se publicó.
- **Push parcial:** inspeccionar logs y images.json parcial; no sobrescribir ni
  borrar silenciosamente. Preparar una nueva versión estable (por ejemplo
  v1.0.1) desde main aprobado. No se garantiza reanudar tres paquetes parcialmente
  publicados con la misma versión.
- **Paquetes privados o descarga temporal fallida:** mantener los digests,
  resolver acceso/conexión y Re-run failed jobs mientras el artefacto siga vigente.
- **Artefactos expirados:** se conservan 14 días. Recuperar registros de forma
  revisada o publicar una versión nueva; no reconstruir y reemplazar v1.0.0.
- **Borrador con adjuntos parciales:** Re-run failed jobs conserva el mismo
  run-id/SHA. finalize verifica los adjuntos existentes y sube solo los ausentes;
  si algún contenido difiere o el borrador es ajeno, se detiene sin sobrescribir.
- **Respuesta perdida al publicar:** si la Release ya está publicada, finalize
  valida propietario lógico (run-id/SHA), tag y los tres adjuntos. Si coinciden,
  termina sin modificar la Release ni sus adjuntos, incluso si es inmutable.
- **Release publicada incompleta:** no intenta añadir archivos; requiere revisión
  humana y una nueva versión si ya es inmutable.
- **Cancelación forzada:** puede interrumpir finally. Los runners son efímeros;
  no se declara limpieza aprobada si no consta resourcesReleased=true.

## Verificación del paquete sin fuentes

Después de una publicación real, el mantenedor puede ejecutar con Python/Docker:

```sh
python distribution/verify_package.py --zip GameStats-Entrega-v1.0.0.zip --evidence docker/evidence/evaluacion-publicada-01
```

El script extrae a un directorio temporal independiente del repositorio,
comprueba su estructura, manifiesto y Compose sin build/mounts/fuentes, usa
configuración Docker vacía, descarga las tres imágenes anónimamente, inicia Go
y Scala con up -d --wait, inspecciona healthy y ejecuta el simulador de esas
imágenes. Comprueba las dos demos 6/50/60, respuestas abiertas/cerradas, tres
negativos, seis rechazos 409, repetición con otro ID y limpieza de sus contenedores
y red. Conserva las imágenes compartidas descargadas; no ejecuta prune.
Reutiliza únicamente las funciones de comprobación de reportes de docker/verify.py,
sin reutilizar su construcción desde fuentes. No usa mocks para acreditar la
instalación. El código fuente del servidor no se monta en ningún contenedor.

## Ensayos locales realizados en esta etapa

- 24 pruebas de controles de publicación y ZIP: versiones, CI exacta, jobs
  omitidos/fallidos, tags existentes, errores de permisos, contexto PR rechazado,
  manifiestos incompletos, plataforma, paths ZIP inseguros, datos de prueba,
  empaquetado determinista y configuración sin fuentes; todas aprobadas.
  Incluyen aprobación manual, orden borrador/adjuntos/publicación, recuperación
  de borrador parcial, Release inmutable ya publicada, adjuntos distintos
  rechazados y referencias de artefactos conservadas al reejecutar jobs fallidos.
- actionlint 1.7.12: ambos workflows válidos; sin ShellCheck/Pyflakes locales.
- Compose de evaluación válido, sin construir ni descargar imágenes previstas.
- 16 pruebas de infraestructura existentes aprobadas, sin cambiar su suite.
- Verificador documental: 30 JSON, 13 pares y 9 mutaciones inválidas correctos.
- ZIP de prueba generado en docker/evidence/academic-preview/, ignorado por Git.

Reproducir el ensayo sintético (usar salida nueva para no sobrescribir):

```sh
python -m unittest discover -s distribution -p "test_*.py" -v
python distribution/release.py pack --lock distribution/images-test.json --output docker/evidence/preview-02 --test-data
docker compose -f distribution/compose.yaml --profile tools config --quiet
```

No se ejecutaron push GHCR, creación de Release, adjuntos ni descarga anónima de
imágenes GameStats publicadas: aún no existen digests de esta entrega obtenidos
por el workflow. La verificación completa de instalación queda pendiente.
Las pruebas locales de ZIP/configuración no se presentan como integración GHCR.
ARM64 no está construido/probado, ni se promete funcionamiento offline.

## Comandos del profesor, solo tras una Release verificada

Extraer el ZIP y abrir una terminal en GameStats-Entrega-v1.0.0:

```sh
docker compose up -d --wait
docker compose ps
docker compose run --rm simulator --run-id evaluacion-01
docker compose run --rm simulator --mode negative --run-id negativos-01
docker compose down
```

Las imágenes se descargan al arrancar; el simulador se descarga cuando se ejecuta
por primera vez. Otro run-id permite otra demo sin reiniciar. Go pierde las
partidas al reiniciarse. Los manuales incluidos explican puerto ocupado, errores
de motor/conexión, arquitectura y consulta HTTP desde navegador/Postman.

## Referencias

- [Registro de contenedores, acceso y etiquetas OCI](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).
- [Publicación con GITHUB_TOKEN](https://docs.github.com/en/packages/managing-github-packages-using-github-actions-workflows/publishing-and-installing-a-package-with-github-actions).
- [API de ejecuciones CI](https://docs.github.com/en/rest/actions/workflow-runs).
- [Releases inmutables y borradores](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository).
- [Semántica de draft/create](https://cli.github.com/manual/gh_release_create).
- [Publicación del borrador](https://cli.github.com/manual/gh_release_edit).
- [download-artifact oficial](https://github.com/actions/download-artifact).
