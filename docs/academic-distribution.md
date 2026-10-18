# Publicación GHCR y distribución académica

## Entrega publicada v1.0.1

La [Release v1.0.1](https://github.com/JusMH/gamestats/releases/tag/v1.0.1)
corresponde al commit `2d32836a8da097de9a43c431d79a2bcea675c5cf`.
La [CI 39114936260](https://github.com/JusMH/gamestats/actions/runs/39114936260)
aprobó sus seis trabajos y la
[publicación 39115113580](https://github.com/JusMH/gamestats/actions/runs/39115113580)
completó aprobación, imágenes, evaluación y finalización. El último paso creó
el borrador, verificó adjuntos y publicó la Release automáticamente.

| Adjunto | Tamaño | SHA-256 verificado |
| --- | --- | --- |
| GameStats-Entrega-v1.0.1.zip | 5 326 bytes | 99a38c758fe824ad5636695ccba0c09e7328d1a14176051c5cfcb631bed9f817 |
| images.json | 734 bytes | 2ae6cff8c3e56fb74d520de98b62698c026f16dc6bb7b1e4fdd32bab42c20980 |
| SHA256SUMS.txt | 95 bytes | cd42db4526af771a1122d4c01670ea81965fae5027ee87d1b8870a3d7e1721d7 |

Los tres adjuntos se descargaron sin credenciales para comprobar sus hashes.
El ZIP está íntegro y su IMAGENES.json coincide con images.json de la Release.
No confundir el adjunto oficial con los archivos automáticos «Source code».

## Imágenes fijadas por digest

Plataforma publicada y evaluada: linux/amd64. Las imágenes tienen tag v1.0.1;
el Compose del paquete también fija su digest, sin compilar fuentes.

| Imagen | Digest del lock publicado |
| --- | --- |
| ghcr.io/jusmh/gamestats-go-ingestion | sha256:93e54c7dd8ec631f41136e9e3a16aa4b882e3037c639fe15340ed91bcd402154 |
| ghcr.io/jusmh/gamestats-scala-analysis | sha256:de3a86f30baa0e769ac259e88f72a134569b23226ff853a1468c93f532b6144d |
| ghcr.io/jusmh/gamestats-simulator | sha256:5f39e6b125039ba1f6e8f7d993b08f3a87dc6a85f85cb54ef71806134675dc07 |

El acceso y la evaluación anónima forman parte del workflow de publicación
exitoso indicado arriba. La revisión local de fuentes construyó otras imágenes
desde el código: esa prueba se distingue de evaluar las imágenes GHCR publicadas.
No se afirma compatibilidad ARM64 ni instalación completamente offline.

## Instalación desde el paquete

Descargar el ZIP oficial de v1.0.1, extraerlo y abrir CMD en su carpeta:

```cmd
docker compose up -d --wait
docker compose ps
docker compose run --rm simulator --run-id evaluacion-01
docker compose run --rm simulator --mode negative --run-id negativos-01
docker compose down
```

Requiere Docker Desktop/motor compatible, Compose y contenedores Linux/amd64.
No requiere Git, Go, Java, Scala, sbt ni Python. La primera descarga necesita
Internet. Go se publica en http://127.0.0.1:8080/health; Scala es interno.
En CMD, para cambiar el puerto antes del arranque: `set "GO_PORT=8081"`.
La demostración normal debe aprobar seis partidas, 50 eventos y 60 consultas.
La evaluación negativa debe aprobar tres comprobaciones.

Usar un run-id nuevo en cada demostración normal mientras siga vivo Go.
Reiniciar Go borra sus partidas. El deployment elegido es local con Compose;
la Release distribuye artefactos, no aloja los servicios en nube.
Las plantillas de los manuales están en [instalación](../distribution/INSTALACION.md)
y [evaluación](../distribution/GUIA-DE-EVALUACION.md).

El contenido del ZIP es:

```text
GameStats-Entrega-v1.0.1/
  compose.yaml
  INSTALACION.md
  GUIA-DE-EVALUACION.md
  IMAGENES.json
```

## Cambios de fuentes posteriores al paquete

La guía [Windows CMD](windows-cmd.md), el wrapper CMD, el ajuste UTF-8 y la
rotación de tres informes se preparan en esta rama. No están incorporados al
ZIP ni a las imágenes v1.0.1, que permanecen intactos. Su disponibilidad en una
entrega binaria requeriría decidir otra publicación; no se genera una aquí.
Para probarlos se ejecutan o construyen las fuentes de esta rama.

## CI y publicación son pasos distintos

[ci.yml](../.github/workflows/ci.yml) se ejecuta para PR hacia main y push a main.
Construye, prueba y verifica integración, pero no publica una nueva Release.
[academic-release.yml](../.github/workflows/academic-release.yml) se inicia
manualmente desde main, con versión, SHA exacto aprobado y confirmación
`PUBLICAR ` seguida de esa versión. Los permisos de escritura están limitados
a los trabajos que publican imágenes y finalizan la Release.

Una publicación futura requiere aprobación humana, CI exitosa del SHA elegido,
imágenes nuevas, evaluación anónima y finalización verificada. No mover tags,
reconstruir versiones publicadas ni reemplazar adjuntos de v1.0.0/v1.0.1 para
incorporar cambios posteriores. Los artefactos temporales de Actions tienen
retención de 14 días; sus enlaces no son una garantía de conservación indefinida.

## Historia de v1.0.0

La [publicación 39105352591, intento 2](https://github.com/JusMH/gamestats/actions/runs/39105352591/attempts/2)
aprobó imágenes y evaluación, pero Finalize terminó en failure con
`Draft creation not confirmed; retry failed job`. v1.0.0 se recuperó manualmente.
Ese hecho sigue documentado en [la auditoría histórica](academic-audit.md).
Las correcciones posteriores y el éxito real de v1.0.1 resuelven la tarea que
aquella auditoría dejó pendiente; no convierten retroactivamente su run en success.

## Comprobaciones de mantenimiento

```cmd
python -m unittest discover -s distribution -p "test_*.py" -v
python -m unittest discover -s docker -p "test_*.py" -v
python docs/fixtures/analysis-v1/check.py
```

Estas pruebas no sustituyen una publicación ni una instalación anónima real.
No se ha cambiado el pipeline ejecutable ni sus permisos en esta rama.
