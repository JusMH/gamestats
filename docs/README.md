# Índice documental de GameStats

Estado auditado: 18 de octubre de 2026. Empezar por el [README principal](../README.md)
y la [Release v1.0.0](https://github.com/JusMH/gamestats/releases/tag/v1.0.0).
El profesor usa el ZIP y sus manuales; el desarrollador utiliza los contratos y
manuales de componentes. La [auditoría](academic-audit.md) relaciona el enunciado
original con las evidencias y registra el fallo conocido de Finalize.

## Inventario completo de Markdown versionable

19 documentos existentes revisados y 3 documentos nuevos: 22 en total. Se excluyen
cachés, dependencias, archivos generados y evidencia ignorada; no son documentación
autoritativa del repositorio. «Corregido» no cambia el carácter normativo del
contrato ni implica cambios de API o perfiles.

| Documento | Clasificación | Resultado de revisión |
| --- | --- | --- |
| [README principal](../README.md) | Manual de entrada y estado | Corregido: instalación publicada, CI/evaluación/Finalize separados. |
| [Arquitectura](architecture.md) | Normativa vigente | Corregido: implementaciones, concurrencia real, límites y deployment local. |
| [Eventos v1](events.md) | Normativa vigente | Corregido: estado de implementación y rechazo de claves ambiguas por Scala; mismo sobre. |
| [Juegos](games.md) | Normativa vigente | Corregido: productores y analizadores ya implementados; alcance conservado. |
| [Plantillas](analysis-templates.md) | Normativa vigente | Corregido: DSL implementado y extensiones claramente futuras. |
| [Métricas y patrones](metrics-and-patterns.md) | Normativa vigente | Corregido: ejecución presente y momento de redondeo. |
| [Integración HTTP](integration-api.md) | Normativa vigente | Corregido: servicio real y verificación existente; mismo protocolo. |
| [Catálogo de fixtures](fixtures/analysis-v1/README.md) | Normativa de ejemplos y manual del verificador | Corregido: 13 fixtures ya ejecutados; JSON y oráculos intactos. |
| [Docker](docker.md) | Manual operativo con registro histórico de Etapa 6 | Corregido: distinguir estado presente y pruebas de aquella etapa. |
| [Distribución académica](academic-distribution.md) | Manual técnico operativo | Corregido: Release/digests reales, recuperación manual y errata del ZIP. |
| [Hoja de ruta](implementation-roadmap.md) | Estado vigente y planificación | Corregido: publicación hecha, Finalize pendiente y modalidad local elegida. |
| [Scala 4A](scala-4a.md) | Histórico de implementación | Conservado: 501 y cobertura parcial son hechos de 4A; banner enlaza estado actual. |
| [Scala 4B](scala-4b.md) | Histórico de implementación | Conservado: resultados de 4B y siguiente tarea en aquel momento, con banner. |
| [Go](../go-ingestion/README.md) | Manual operativo | Corregido: versiones declaradas/probadas, CI y distribución. |
| [Scala](../scala-analysis/README.md) | Manual operativo | Corregido: estado actual y Temurin 21 de CI frente a patch de Docker. |
| [Simulador](../simulator/README.md) | Manual operativo con evidencia de Etapa 5 | Corregido: CI/race posterior y distribución; escenarios conservados. |
| [Workflows](../.github/workflows/README.md) | Manual operativo de CI/publicación | Corregido: CI verde y publicación parcialmente fallida, sin cambiar YAML. |
| [Instalación del evaluador](../distribution/INSTALACION.md) | Plantilla de manual operativo | Identificada como fuente de futuros paquetes; adjunto v1.0.0 intacto. |
| [Guía del evaluador](../distribution/GUIA-DE-EVALUACION.md) | Plantilla de manual operativo | Identificada como fuente de futuros paquetes; adjunto v1.0.0 intacto. |
| [Integrar nuevos juegos](integrating-new-games.md) | Guía técnica operativa nueva | Perfil score.player y productor externo; Prolog explícitamente FUTURO. |
| [Auditoría académica](academic-audit.md) | Informe de auditoría nuevo | Matriz del enunciado, evidencias, límites y verificaciones. |
| [Este índice](README.md) | Navegación e inventario nuevo | Clasificación completa y fuentes de verdad. |

## Orden de lectura y fuentes de verdad

1. Instalación: ZIP oficial y manuales incluidos. Para su estado real y erratas,
   [distribución académica](academic-distribution.md).
2. Contratos: eventos → integración HTTP → plantillas → resultados; games define
   únicamente los datos de los juegos de demostración. Los fixtures concretan
   esos contratos y las pruebas comparan resultados, no solo sintaxis.
3. Operación desde fuentes: README de cada módulo y guía Docker. CI/publicación
   explican automatización y diagnósticos; no alojan el sistema en nube.
4. Extensión y defensa: guía de nuevos juegos, arquitectura y matriz académica.
5. Historia: informes 4A/4B y secciones de resultados de etapas anteriores.
   Sus exclusiones no sustituyen el estado vigente de la hoja de ruta.

## Duplicación y conservación

No se propone eliminar ningún documento. Los comandos breves repetidos en README,
Docker e instalación cumplen públicos distintos; los contratos enlazados son la
fuente detallada, no copias alternativas. Los informes 4A/4B contienen decisiones,
cobertura y políticas transitorias que no están completamente duplicadas.
Las introducciones obsoletas se corrigieron en lugar de borrar esa trazabilidad.
El ZIP y tag ya publicados no se reescriben con esta revisión.
