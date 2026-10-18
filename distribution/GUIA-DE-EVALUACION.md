# Evaluación de GameStats {{VERSION}}

El simulador es un productor HTTP independiente. Genera acontecimientos
hipotéticos; no ejecuta videojuegos gráficos ni envía estadísticas calculadas.
Go recibe y valida los eventos, coordina partidas y toma snapshots. Scala
interpreta perfiles componibles y calcula estadísticas y patrones.

## Demostración principal

Con los servicios iniciados según INSTALACION.md:

```sh
docker compose run --rm simulator --run-id evaluacion-01
```

La demostración crea dos partidas por juego, con jugadores alice y bob:

| Juego | Evidencia que se evalúa |
| --- | --- |
| Gato | Victoria de alice en fila 2 y empate; movimientos por jugador/partida; reconstrucción del tablero y primera línea ganadora. |
| Snake | Manzanas, puntos incrementales, promedios y muertes; una partida con tres manzanas en diez segundos y otra sin esa ráfaga. |
| Carreras | Vueltas, suma y promedio de tiempos y duración; varias capacidades del mismo perfil sin reglas de carreras en Go. |

Ejemplos finales: Snake positivo da 24 puntos y promedio 8 a alice, con una
ráfaga; Snake negativo da 45 y promedio 15, sin ráfaga. race-sprint da tres
vueltas y 30000 ms a alice, y dos vueltas y 20000 ms a bob; duración 32 s.
Gato con victoria registra cinco movimientos y una línea; empate registra nueve.

Cada partida progresa en cinco fases y consulta dos veces por fase: seis
partidas, 50 acontecimientos aceptados y 60 consultas analíticas. Los logs
ACCEPTED, RESULT y PHASE muestran progreso concurrente. Las barreras permiten
que diferentes partidas estén abiertas simultáneamente y conservan el orden
dentro de cada partida.

RESULT distingue resultados provisional y final; una duración sin cierre
aparece unavailable, sin inventar un fin. Las consultas repetidas deben coincidir,
sin duplicar contadores. El simulador compara cantidades, promedios, sujetos,
estados, procedencia, basis y patrones contra expectativas independientes.
Solo HTTP 200 no basta: una discrepancia produce ERROR y salida distinta de cero.
El resumen ordena partidas para que la planificación concurrente no altere su lectura.

## Consultas desde el anfitrión

Después de la demo, abra en el navegador (o use GET desde Postman):

- http://127.0.0.1:8080/health
- http://127.0.0.1:8080/matches/snake/evaluacion-01-snake-positive
- http://127.0.0.1:8080/results/snake/evaluacion-01-snake-positive
- http://127.0.0.1:8080/results/tic-tac-toe/evaluacion-01-ttt-win
- http://127.0.0.1:8080/results/racing/evaluacion-01-race-sprint

Cambie el puerto si configuró GO_PORT, y el prefijo si utilizó otro run-id.
GET /results pide análisis nuevo a Scala sin cerrar ni acumular estadísticas.
Los resultados provisionales se observan en los logs durante la demo; después
del cierre, las consultas corresponden al conjunto definitivo aceptado.

## Casos negativos e identificadores

```sh
docker compose run --rm simulator --mode negative --run-id negativos-01
```

Deben pasar tres comprobaciones: sobre inválido rechazado sin mutación;
datos de dominio inválidos aceptados por Go con salidas afectadas invalid en
Scala; juego desconocido admitido con arrays analíticos vacíos.

Repetir `evaluacion-01` en el mismo proceso Go debe fallar con HTTP 409 y
salida 1, porque sus seis partidas ya existen. Un timeout u otro error no
acredita esta prueba. Para una nueva demo use:

```sh
docker compose run --rm simulator --run-id evaluacion-02
```

## Diagnóstico y requisitos académicos

```sh
docker compose ps
docker compose logs --tail 100 go-ingestion scala-analysis
docker compose down
```

Estos pasos demuestran separación Go/Scala por HTTP, concurrencia, análisis
funcional multijuego, estadísticas por jugador y partida, composición de
capacidades, relaciones multievento, secuencias, resultados en vivo/finales y
verificación automatizada de errores. Las imágenes y el ZIP se preparan mediante
GitHub Actions; no se compila en la computadora del evaluador.

No se ofrece persistencia histórica, interfaz gráfica, SSE/WebSockets ni
servicio alojado en la nube. Cerrar Go pierde las partidas. La evaluación local
con Docker es la modalidad de entrega; no equivale a deployment remoto.
