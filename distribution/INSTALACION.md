# Instalar GameStats {{VERSION}}

<!-- Plantilla del repositorio: el generador sustituye VERSION. Las correcciones
     de esta rama no modifican los manuales ya publicados bajo v1.0.0. -->

Necesita Docker Desktop instalado e iniciado, con contenedores Linux y Docker
Compose 2.20 o posterior. Esta entrega se verifica para Linux/amd64 (PC Intel/AMD).
No necesita Go, Scala, Java, sbt, Python ni Git. La primera descarga requiere Internet.

1. Extraiga todo el ZIP y abra una terminal dentro de la carpeta extraída.
2. Inicie los servicios:

```sh
docker compose up -d --wait
```

3. Compruebe que Go y Scala aparecen como healthy:

```sh
docker compose ps
```

También puede abrir http://127.0.0.1:8080/health en su navegador: debe mostrar
`{"status":"ok"}`. Esto comprueba Go; la demostración siguiente verifica el
análisis real Go → Scala.

4. Ejecute la demostración multijuego:

```sh
docker compose run --rm simulator --run-id evaluacion-01
```

Debe finalizar con PASS y seis partidas verificadas. Consulte
GUIA-DE-EVALUACION.md para revisar estadísticas y casos negativos.

5. Detenga el sistema:

```sh
docker compose down
```

Go guarda las partidas en memoria: reiniciar o recrear Go las elimina.
Las imágenes descargadas quedan en la caché local de Docker.

## Si aparece un error

- **No conecta con Docker:** abra Docker Desktop y espere a que el motor esté listo.
- **No descarga imágenes:** compruebe Internet y el acceso a ghcr.io. Reintente
  `docker compose --profile tools pull`. Si aparece denied/unauthorized, contacte
  al equipo: las tres imágenes de la entrega deben permitir acceso sin login.
- **Puerto 8080 ocupado:** cree un archivo `.env` en esta carpeta con
  `GO_PORT=8081` y repita el arranque. Consulte entonces http://127.0.0.1:8081/health.
- **HTTP 409 al repetir una demo:** utilice otro identificador, por ejemplo
  `--run-id evaluacion-02`. No se reabren partidas cerradas.
- **Arquitectura incompatible:** esta entrega solo acredita Linux/amd64. ARM64
  y la emulación en equipos ARM no están verificados; consulte al equipo.
- **Alguna imagen sigue fallando:** ejecute `docker compose logs --tail 100` y
  comparta el error. No es necesario compilar ni instalar herramientas adicionales.

Las referencias de imagen están fijadas por digest; no edite sus valores.
