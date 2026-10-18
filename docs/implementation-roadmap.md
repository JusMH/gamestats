# MVP académico y hoja de implementación

## Estado actual verificable

| Etapa | Estado | Evidencia |
| --- | --- | --- |
| Go 1–3: ingestión y coordinación | Implementado | Sobre v1, memoria por juego/partida, lifecycle, concurrencia, snapshots y puente HTTP bajo demanda. |
| Scala 4A: motor funcional | Implementado | Perfiles declarativos, count/sum/avg, duración, composición y HTTP. |
| Scala 4B: patrones y especialización | Implementado | Ventanas temporales, tablero Gato y 13 fixtures completos, motor puro/HTTP/integración. |
| Simulador 5 | Implementado | Seis partidas propias, tres juegos, productores HTTP, fases concurrentes y verificación independiente en vivo/final. |
| Docker y Compose | Pendiente | Próxima tarea delimitada. |
| CI/CD y deployment | Pendiente | Etapas posteriores obligatorias. |

La implementación 5 no modificó lógica Go/Scala, perfiles ni fixtures.
Documentación operativa: [Go](../go-ingestion/README.md),
[Scala](../scala-analysis/README.md), [verificación 4B](scala-4b.md)
y [simulador](../simulator/README.md).

## Requisitos académicos cubiertos y pendientes

| Requisito | Evidencia actual |
| --- | --- |
| Ingestión, flujo, memoria y concurrencia Go | Pruebas de eventos, partidas, snapshots y HTTP. |
| Servicio Scala independiente | POST /analyze y GET /health; puente Go real. |
| Programación funcional | Transformaciones puras, agrupaciones, folds inmutables y errores explícitos. |
| Métricas configurables | count/sum/avg por jugador/partida, sin condicionales por juego en el motor. |
| Composición | Racing combina métricas/duración; Snake agrega ventanas; Gato agrega especialización. |
| Análisis multievento | Inicio-fin, inicio-muerte, ternas de manzanas y tablero reconstruido. |
| Secuencias | Ventana greedy temporal y primera línea ganadora. |
| Análisis abierto/final | GET /results repetidos sin acumulación; provisional/final/unavailable/invalid. |
| Fuente externa simulada | Módulo Go separado, dos partidas por juego y dos jugadores por partida. |
| Simultaneidad | Workers por partida y barreras; seis solicitudes HTTP simultáneas demostradas. |
| Pruebas de contratos e integración | 13 fixtures Scala; escenarios propios del simulador con servicios reales. |
| Docker y ejecución integrada reproducible | Pendiente de contenerización; hoy existe script local Windows. |
| CI/CD y deployment funcional | Pendientes. |
| Documentación y defensa | Contratos, perfiles, fixtures, README e informes reproducibles locales. |

El MVP académico completo aún requiere contenedores, CI/CD, despliegue y su
validación final. No se da por concluido por disponer del simulador.

## Verificación ejecutada en implementación 5

- Simulador: compilación y go vet correctos; 31 funciones de prueba,
  90 casos terminales aprobados, sin contar dos veces los subtests padre.
- Servidor Go: go test ./... y go vet ./... correctos.
- Scala: compilación y 143 pruebas aprobadas, incluidos los 13 fixtures.
- Verificador documental: 30 JSON, 13 pares y 9 mutaciones inválidas.
- Integración con procesos independientes: seis partidas, 50 eventos,
  60 consultas verificadas, 48 abiertas y 12 cerradas; pico de seis
  solicitudes simultáneas.
- Tres comprobaciones negativas y seis rechazos 409 al repetir el run-id.
- go test -race del simulador pendiente por entorno: CGO deshabilitado
  y ausencia de gcc/clang en PATH. No se declara aprobado.

El simulador genera escenarios propios y expectativas explícitas: no usa
los fixtures como un sustituto de sus productores. Las pruebas HTTP
guionizadas comprueban la orquestación; la integración ejecutada realiza
realmente los cálculos en Scala.

## Próxima tarea precisa: contenedores y Docker Compose

1. Revisar los requisitos de compilación y ejecución: Go, JDK 21, Scala/sbt,
   perfiles locales y variables de entorno ya existentes.
2. Crear builds reproducibles de Go, Scala y simulador, con versiones fijadas
   y sin copiar cachés, evidencia ni secretos del equipo de desarrollo.
3. Configurar Compose con Go y Scala independientes, red interna y perfiles
   montados o incluidos de manera explícita. Mantener contratos y endpoints.
4. Ejecutar el simulador como cliente finito de la red Compose, pasando
   go-url, run-id, verificación y ruta de evidencia.
5. Definir salud y readiness sin asumir que orden de arranque equivale a
   servicio listo; conservar límites/errores HTTP y cancelación.
6. Verificar escenarios normales/negativos, resultados abiertos/finales y
   repetición con 409 en los servicios reales dentro de contenedores.
7. Documentar arranque, inspección, logs, ejecución del simulador y parada;
   indicar que reiniciar Go pierde su almacenamiento en memoria.

Criterio de salida: levantar el sistema con Compose y obtener la misma
demostración verificada, sin instalaciones locales de Go/Scala para ejecutarla.
No incluir CI/CD, deployment ni bases de datos dentro de esa tarea.

Después: CI con pruebas de los tres módulos y contratos; publicación y
deployment acordados con el equipo; verificación remota y documentación final.

## Extensiones fuera del alcance actual

min/max para mejor vuelta, nuevas unidades, capacidades adicionales y nuevos
perfiles pueden estudiarse después; no están incluidos por ser sencillos.
Nuevos mapeos de eventos a count/sum/avg ya funcionan mediante perfiles.

No se incorporan ajedrez/damas completos, motor universal de reglas,
interfaces gráficas, registro dinámico por API, persistencia distribuida,
Kafka, torneos/equipos, procesamiento incremental avanzado ni WebSockets/SSE.

## Riesgos y decisiones pendientes

- Go retiene todos los eventos en memoria y no comparte estado entre procesos.
  Definir límites/retención antes de escalar; no asumir persistencia.
- No hay reintentos idempotentes, reapertura ni exactamente una vez.
  El simulador usa run-id distinto para nuevas demostraciones.
- Snapshots completos y polling tienen costo creciente. Medir antes de
  introducir caché, procesamiento incremental o mayor frecuencia.
- La versión activa del perfil puede cambiar resultados incluso cerrados;
  source.profileVersion debe permanecer visible y las versiones ser inmutables.
- El puente traduce fallos HTTP de Scala a 502; diagnósticos públicos más
  específicos requerirían una tarea separada, no modificaciones en el simulador.
- Gato verifica la evidencia acordada, sin certificar todas las reglas.
  Si la rúbrica exige más legalidad, deberá acordarse explícitamente.
- La detección de carreras Go debe ejecutarse en un entorno con CGO y
  compilador C compatibles; las pruebas concurrentes actuales no la sustituyen.
- La plataforma de despliegue, credenciales y políticas de acceso deben
  acordarse al preparar deployment, sin inventar infraestructura ahora.