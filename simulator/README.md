# Simulador multijuego GameStats — implementación 5

Aplicación Go independiente que produce hechos hipotéticos y utiliza exclusivamente
el contrato HTTP público. No importa paquetes del servidor, no ejecuta videojuegos
y no calcula las estadísticas que envía a Scala: solo envía eventos a Go y consulta
los análisis devueltos por Go.

Estado verificado: seis partidas propias, 50 eventos aceptados y 60 resultados
comparados con expectativas independientes, utilizando Go y Scala reales.
La ejecución Docker se documenta al final; CI/CD y deployment siguen pendientes.

## Requisitos y arranque

Go 1.22 o posterior para el simulador y la ingestión. Verificado con Go 1.26.5
en Windows/amd64. Solo biblioteca estándar, sin dependencias externas.
Scala utiliza JDK 21 y su wrapper local sbt; no necesita un IDE ni una
instalación global de Scala/sbt. Ver [preparación Scala](../scala-analysis/README.md).

En tres terminales, desde la raíz del repositorio:

~~~powershell
# Terminal 1: Scala, puerto 9000
cd scala-analysis
.\sbtw.ps1 run
~~~

~~~powershell
# Terminal 2: Go, puerto 8080
cd go-ingestion
$env:SCALA_SERVICE_URL = 'http://127.0.0.1:9000'
go run ./cmd/server
~~~

~~~powershell
# Terminal 3: simulador
cd simulator
go run ./cmd/simulator --run-id defensa-01 --report evidence/defensa-01.json
~~~

En Linux/macOS se puede iniciar Scala con sbt run y establecer la variable
SCALA_SERVICE_URL en la shell correspondiente. La aplicación Go es portable;
el script que automatiza los tres procesos está escrito para Windows/PowerShell.

Compilación independiente, desde simulator:

~~~powershell
go build -o bin/simulator.exe ./cmd/simulator
.\bin\simulator.exe --help
~~~

## Interfaz

| Opción | Por defecto | Significado |
| --- | --- | --- |
| --go-url | http://127.0.0.1:8080 | URL HTTP(S) base de Go, admite prefijo de ruta. |
| --run-id | demo | Prefijo reproducible de partidas y eventos. |
| --mode | demo | demo: seis partidas normales; negative: tres comprobaciones separadas. |
| --verify | true | Compara expectativas; false solo observa y lo declara expresamente. |
| --concurrency | 6 | Máximo de partidas activas por lote, entre 2 y 16. |
| --timeout | 10s | Timeout total por solicitud, incluida lectura del cuerpo. |
| --max-duration | 2m | Plazo total; cancelación ordenada de todas las partidas. |
| --report | vacío | Ruta opcional de evidencia JSON con resultados, errores y pico de solicitudes. |
| --help | | Muestra ayuda. |

Código de salida: 0 si se completó el modo elegido; 1 ante error HTTP,
discrepancia, cancelación o fallo de escritura; 2 ante argumentos inválidos.
Con --verify=false no se declara verificación analítica exitosa, aunque se
siguen comprobando aceptaciones, identidad, estado, ingestión y repetición.

El run-id admite 1..48 caracteres ASCII: empieza con letra o dígito, seguido
de letras, dígitos, guion o guion bajo. No usa la hora real ni genera UUID.

## Escenarios propios

Todos usan los jugadores alice y bob; compartir IDs permite comprobar que sus
estadísticas permanecen separadas entre partidas. Los escenarios se construyen
como datos nuevos en memoria y no leen ni reproducen los fixtures de docs/.

| Nombre relativo | Hechos y resultado final esperado |
| --- | --- |
| ttt-win | Cinco movimientos alternados: alice completa la fila 2; conteos 3/2; victoria y una línea; duración 12 s. |
| ttt-draw | Nueve movimientos alternados en casillas distintas; ninguna línea; conteos 5/4; empate; duración 30 s. |
| snake-positive | alice consume incrementos 4, 8 y 12 en segundos 1, 5 y 9: 24 puntos, media 8, una ráfaga. bob consume 6 y 14: 20, media 10, sin ráfaga. Mueren en segundos 15/18; duración 20 s. |
| snake-negative | alice consume 5, 15 y 25 en segundos 1, 12 y 23: 45, media 15, sin ráfaga. bob consume 7 y 13: 20, media 10. Mueren en segundos 26/28; duración 30 s. |
| race-sprint | alice: tres vueltas de 8000, 10000 y 12000 ms; total 30000 y media 10000. bob: dos de 9000 y 11000; total 20000 y media 10000. Duración 32 s. |
| race-endurance | alice: tres de 11000, 13000 y 9000 ms; total 33000 y media 11000. bob: dos de 12000 y 14000; total 26000 y media 13000. Duración 35 s. |

Las duraciones de vuelta concuerdan con los instantes del productor para cada
corredor. LAP_COMPLETED comunica un hecho detectado por el productor.
Los puntos de Snake son incrementos por manzana, nunca puntuación acumulada.
Los cierres llevan data:{}; no declaran ganador, puntuaciones ni estadísticas.

Los timestamps parten de 2026-10-20T12:00:00Z y usan desplazamientos enteros
fijos. Los IDs son run-id + nombre de escenario; los eventos añaden -e01,
-e02, etc. Igual configuración produce los mismos hechos y expectativas,
independientemente de la planificación de goroutines. No hay seed porque no
hay generación aleatoria.

## Concurrencia y consultas en vivo

Una goroutine por partida, con lotes limitados por --concurrency.
Dentro de cada partida se espera la respuesta accepted de cada POST antes
de enviar el siguiente evento. El cierre solo se envía tras todas las
aceptaciones anteriores.

Cada escenario tiene cinco checkpoints:
inicio, dos progresos parciales, último estado abierto y cierre.
Una barrera espera que todas las partidas del lote completen cada checkpoint
antes de permitir el siguiente. Así todas empiezan y consultan abiertas antes
de que alguna pueda cerrarse. Partidas distintas progresan en paralelo;
el orden global de sus aceptaciones e impresiones puede cambiar.

No se usan sleeps como mecanismo de coordinación. Los canales y WaitGroup
coordinan fases; un mutex protege la impresión y contadores atómicos miden
solicitudes activas. Un error queda en el resumen de su partida y las demás
continúan. Las barreras conservan su número de participantes incluso después
de un error o cancelación, evitando dejar workers bloqueados.

En cada checkpoint:
1. Se envía únicamente el nuevo sufijo de eventos.
2. GET /matches comprueba estado, cantidad y orden exacto de IDs aceptados.
3. Se consulta GET /results dos veces con el mismo snapshot.
4. Se comparan expectativas y equivalencia de las dos respuestas.

Ejemplo de race-sprint para alice, realmente verificado:

| Fase | Vueltas | Media ms | Estado analítico |
| --- | --- | --- | --- |
| Inicio | 0 | unavailable/no_samples | provisional para conteo |
| Primer progreso | 2 | 9000 | provisional |
| Segundo progreso | 2 | 9000 | provisional |
| Último estado abierto | 3 | 10000 | provisional |
| Cierre | 3 | 10000 | final |

Mientras tanto bob progresa de 0 a 1 y luego 2 vueltas. El total de partida
acaba en 5. Duración de partida permanece unavailable/awaiting_finish hasta
el cierre, cuando vale 32 s. No se usa el reloj de consulta como fin ficticio.
Repetir GET no cierra partidas ni acumula contadores.

## Verificación independiente y presentación

Las expectativas por checkpoint son tablas literales de cantidades, sumas,
promedios, duraciones, resultados de tablero y ocurrencias. Se preparan antes
de iniciar la red; ninguna expectativa se obtiene copiando la respuesta Scala.

Se verifican todas las filas esperadas, su cantidad y orden, IDs, sujetos,
unidades, estados, reasons, source, profileVersion y basis. Los patrones
comprueban count, eventos concretos, tiempos y detalles de la línea ganadora.
Números JSON se comparan con math/big.Rat para evitar pérdida por float64.
Las métricas pertenecen al perfil aprobado 1.0.0; cambiarlo requiere revisar
explícitamente las expectativas.

Durante la ejecución se muestran ACCEPTED, RESULT, PHASE y ERROR.
Cada RESULT muestra datos y estados relevantes; el reporte JSON conserva
todas las filas conocidas del contrato. El resumen final se ordena por
gameId/matchId y muestra aceptaciones, consultas, verificación y errores.
El orden concurrente del log no se usa como resultado determinista.

## Casos negativos separados

~~~powershell
go run ./cmd/simulator --mode negative --run-id negativos-01 --report evidence/negativos-01.json
~~~

- Sobre inválido: se rechaza un inicio con versión 2 y se comprueba que no
  creó partida; después se rechaza un evento sin ID y se comprueba que la
  partida existente no cambió. También se verifican duplicado y evento nuevo
  después del cierre: 409 y estado intacto.
- Dominio inválido: points como texto tiene sobre válido y es aceptado.
  Suma/promedio de alice son invalid/invalid_number; conteos y una terna
  temporal siguen válidos. Se comprueba en abierto y cerrado.
- Juego desconocido unprofiled-demo: eventos con data anidada son aceptados;
  resultados abiertos/cerrados con arrays vacíos, sin métricas inventadas.

Son partidas con nombres distintos de la demostración normal. Este modo
siempre verifica; no admite --verify=false.

## Repetir una demostración

Go retiene partidas en memoria. Una nueva ejecución debe usar otro --run-id:

~~~powershell
go run ./cmd/simulator --run-id defensa-02
~~~

Reutilizar el mismo run-id en el mismo proceso Go falla con HTTP 409 y salida
1, aunque los eventos sean idénticos. No hay borrado, reapertura ni reintentos
automáticos. Tras reiniciar Go se puede repetir exactamente la misma ejecución.
Un fallo de red puede dejar un prefijo aceptado: no se intenta adivinar su
estado ni reanudarlo automáticamente. Utilizar otro run-id o reiniciar el
servidor durante una demostración controlada.

## Pruebas

Desde simulator:

~~~powershell
go build ./...
go test ./...
go vet ./...
go test -race ./...
~~~

Resultado ejecutado: **31 funciones de prueba, 90 casos terminales aprobados,
0 fallos**. El conteo de casos no duplica las suites padre de subtests.
Cubren determinismo, sobres, lifecycle, orden, aislamiento, barreras con seis
solicitudes simultáneas, límites de concurrencia, fallos independientes,
errores HTTP, redirecciones, respuestas grandes, timeouts/cancelación,
errores analíticos con HTTP 200, IDs repetidos, estados y precisión numérica.

Las pruebas httptest usan respuestas guionizadas para probar transporte,
orquestación y verificación. No calculan estadísticas ni sustituyen la
integración con Scala real. Una primera prueba de timeout detectó que el
handler de prueba debía consumir el cuerpo para observar la desconexión;
se corrigió y toda la suite final pasó.

go test -race no pudo ejecutarse: el entorno tiene CGO_ENABLED=0 y no hay
gcc/clang en PATH. Se conserva el comando para un entorno con los requisitos;
no se declara aprobada la detección de carreras.

### Integración real automatizada en Windows

Con puertos 8080 y 9000 libres, desde simulator:

~~~powershell
.\scripts\check-live.ps1
# Puerto Scala alternativo, si es necesario:
.\scripts\check-live.ps1 -RunID otra-demo -ScalaPort 9100
~~~

El script compila los tres componentes; obtiene el classpath mediante sbt,
inicia Go y Scala como procesos independientes y ejecuta el simulador real.
Comprueba demostración, repetición con 409 y casos negativos. Sus variables
de entorno son locales al proceso y se restauran al terminar.
Si un puerto está ocupado, se detiene sin tocar el servicio existente.
Solo detiene los procesos que él mismo inició, también ante fallos.

Evidencia generada, ignorada por Git:
- evidence/live/demo.json y demo.log: seis partidas y resultados completos.
- evidence/live/duplicate.json: seis rechazos 409 con salida 1 esperada.
- evidence/live/negative.json: tres comprobaciones negativas aprobadas.
- evidence/live/summary.json y logs separados de ambos servicios.

Resultado real ejecutado: **50 eventos**, **6 partidas**, **60 consultas**
(**48 abiertas y 12 cerradas**), **6 solicitudes simultáneas**, **3 casos
negativos correctos** y **6 reejecuciones rechazadas**.
Los procesos propios terminaron después de la verificación.
Además pasaron pruebas/vet de Go, las 143 pruebas Scala y el verificador
documental de 30 JSON, 13 pares y nueve mutaciones inválidas.
No se modificaron Go, Scala, perfiles ni fixtures de referencia.

## Estructura y extensión

~~~text
simulator/
  go.mod
  cmd/simulator/
    main.go, main_test.go
  internal/simulation/
    model.go          Sobre público y resultados tipados
    scenarios.go      Productores de Gato, Snake y carreras
    expected.go       Construcción de expectativas declaradas
    client.go         HTTP con contexto, límites y sin reintentos
    verify.go         Comparación exacta y diagnósticos
    runner.go         Workers por partida y barreras
    negative.go       Comprobaciones negativas independientes
    *_test.go         Generación, cliente, verificador y orquestación
  scripts/check-live.ps1
  bin/                Ejecutables locales ignorados
  evidence/           Evidencia local ignorada
~~~

Para añadir un escenario, crear un productor que devuelva Scenario con hechos
y cinco checkpoints de expectativas explícitas, añadirlo a Scenarios y probar
su determinismo, orden y resultado. Para otro juego, definir sus hechos y
un perfil analítico con las capacidades existentes; solo una regla nueva
especializada requiere implementación Scala. El simulador y los servicios
se siguen comunicando por HTTP; no se añaden imports internos entre módulos.

## Limitaciones y siguiente etapa

Es una demostración acotada y reproducible, no un generador de carga ni un
simulador físico. Las barreras favorecen mostrar estados simultáneos sobre
maximizar throughput; un timeout de una partida retrasa esa fase del lote.
Cada partida del simulador debe tener un único productor durante la prueba.

La verificación depende de las versiones de perfiles documentadas.
No hay persistencia, reanudación, idempotencia, seed, registro dinámico,
notificaciones push ni garantías exactamente una vez. CI/CD y deployment
siguen pendientes.

La implementación 6 incorpora Docker Compose verificado para los tres componentes,
como se describe a continuación. Siguiente tarea: CI/CD.

## Ejecución como tarea Docker

Desde la raíz, con Docker y Compose:

~~~sh
docker compose up -d --build
docker compose run --rm simulator --run-id docker-demo-01
docker compose run --rm simulator --mode negative --run-id docker-negative-01
docker compose run --rm simulator --run-id docker-demo-02
~~~

No se necesita Go en el anfitrión. La imagen multietapa contiene solo el
ejecutable y runtime Alpine; usuario 10001:10001. Compose mantiene
--go-url http://go-ingestion:8080 en entrypoint, incluso cuando run reemplaza
el command con los argumentos proporcionados. Se conservan todos los flags
originales, incluidos --report y --verify; la verificación está habilitada
en la demostración y el código de salida se propaga sin wrappers.

El perfil tools excluye el simulador del up normal. Se espera a Go y Scala
healthy antes de ejecutar la tarea. /reports permite evidencia interna,
sin volumen persistente; con --rm se elimina al finalizar. Para guardar
reportes y verificar duplicados, nuevo ID y limpieza, usar opcionalmente:

~~~sh
python docker/verify.py
~~~

[Guía Docker completa, variables y resultados ejecutados](../docs/docker.md).
