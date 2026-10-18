# Implementación Scala 4A: decisiones y verificación

> Documento histórico de 4A. La política transitoria 501 quedó eliminada
> en [4B](scala-4b.md); allí se documenta el cierre de 4B. El [índice vigente](README.md) y la
> [auditoría final](academic-audit.md) describen las etapas posteriores.

Implementación real del subconjunto genérico de la Etapa 1, sin cambios en
Go, perfiles ni fixtures aprobados. Ver [ejecución y versiones](../scala-analysis/README.md).

## Disponible

- HTTP POST /analyze y GET /health, host/puerto configurables.
- Validación de snapshots y perfiles con modelos inmutables.
- Registro versionado de capacidades y DSL JSON tipado.
- event-metrics/v1: count, sum, avg por jugador/partida.
- duration/v1: inicio-fin por partida e inicio-evento final por jugador.
- Composición determinista, estados y procedencia de resultados.
- Repetición sin acumulación, sin estado analítico entre peticiones.
- Juego sin perfil: 200, metrics y patterns vacíos.

Los perfiles pueden incorporar nuevos gameId y métricas con las capacidades
implementadas sin cambiar el ejecutor. Racing demuestra la composición de
event-metrics y duration. Las pruebas agregan un juego y una métrica nuevos
en memoria, sin reescribir archivos de configuración aprobados.

## Política transitoria de capacidades pendientes

El contrato definitivo incluye temporal-window/v1 y tic-tac-toe-line/v1.
4A reconoce y valida sus configuraciones, pero no ejecuta esas reglas.
El perfil completo que las seleccione recibe HTTP 501:

~~~json
{"error":"capability_not_implemented"}
~~~

Esto afecta los perfiles Snake y Gato, no los juegos sin perfil.
No se finge ausencia de configuración, no se omiten plantillas ni se
devuelven estadísticas parciales con HTTP 200. Un ID realmente desconocido
sigue impidiendo el arranque por error de configuración.

Esta es la única excepción transitoria al registro completamente ejecutable
del diseño final. El puente Go mantiene su política: un 501 de Scala se
presenta como 502, sin exponer detalles. No hace falta cambiar Go.
La próxima etapa debe registrar los ejecutores pendientes y eliminar el
rechazo para esos perfiles.

## Precisiones de implementación

La suma conserva todos los dígitos mediante suma decimal exacta.
Promedios y segundos fraccionarios se redondean una vez a seis decimales
HALF_EVEN al preparar la salida; no hay dependencia entre métricas ni
composición de subtotales redondeados.

La API exige un snapshot completo y coherente antes de buscar el perfil.
El roster inválido provoca 400. Un jugador ausente o desconocido en eventos
seleccionados invalida las métricas player dependientes: sin sujeto fiable,
se marcan las filas de ese alcance; las métricas match independientes pueden
seguir siendo válidas. No se crea una fila para un jugador inventado.

Claves JSON repetidas se rechazan durante la lectura, incluso dentro de
data: HTTP 400, sin interpretación ambigua. No se cambia la ingestión Go.
Los límites operativos se detallan en el README; un JSON admisible por Go
puede superar los límites del servicio analítico y recibir error explícito.
Los desplazamientos RFC3339 hasta +/-23:59 se normalizan sin depender de
la restricción +/-18:00 de ZoneOffset; las fracciones mayores que nanosegundos
se truncan como en Go. Las capacidades 4A no necesitan inferir secuencias;
las políticas temporales de ventanas y tablero siguen pendientes de 4B.

## Verificaciones ejecutadas

| Comprobación | Resultado |
| --- | --- |
| sbt compile | Correcto, Scala 3.3.7 sobre JDK 21 |
| sbt test | 91 pruebas aprobadas |
| Fixtures completos motor y HTTP | race-start, race-live-2, race-live-3, race-final, unknown-game |
| HTTP real concurrente | 16 partidas distintas con respuestas aisladas |
| Go test y vet | Correctos |
| Oráculo Python de Etapa 1 | 30 JSON, 13 pares y 9 mutaciones inválidas correctos |
| Puente real Go → Scala | Cuatro snapshots de carreras consultados dos veces: ocho respuestas completas iguales a fixtures |
| Código Go, perfiles y fixtures | Sin cambios respecto a la base de esta rama |

La prueba del puente levantó únicamente procesos propios y los detuvo
al terminar. No usó un mock analítico.

Los casos ttt-win, ttt-draw, ttt-open y los cinco snake-* no han pasado como
análisis completos: sus ejecutores son de 4B. Sus pruebas actuales comprueban
el rechazo explícito, no el resultado analítico. La duración inicio-muerte
sí se prueba aisladamente con una configuración de prueba independiente.

## Próxima tarea 4B

1. Implementar y registrar temporal-window/v1 como función pura con
   (timestamp UTC, índice de aceptación), límite inclusivo y consumo greedy.
2. Implementar tic-tac-toe-line/v1 separado del ejecutor; ampliar la
   procedencia para analyzerId y el valor estructurado de board.result.
3. Conservar los perfiles aprobados. Comparar los ocho expected pendientes
   completos, tanto en el motor como mediante HTTP.
4. Agregar casos de solapamiento largo, timestamps simultáneos/equivalentes,
   eventos tardíos, tablero inválido, doble ganador y partidas abiertas.
5. Repetir integración Go y comprobar que los perfiles ya ejecutables
   dejan de responder 501. No añadir simulador, Docker ni motor universal.
