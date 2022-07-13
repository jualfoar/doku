# about-anchor
# About
Doku is a simple, lightweight web-based application that allows you to monitor Docker disk usage in a user-friendly manner.

The Doku displays the amount of disk space used by the Docker daemon, splits by images, containers, volumes, and builder cache.

# getting-doku-anchor
# Getting Doku

Doku is a very small Docker container (6 MB compressed). Pull the latest release from the index:

    docker pull amerkurev/doku:latest

# using-doku-anchor
# Using Doku

The simplest way to use Doku is to run the Docker container. Mount the Docker Unix socket with `-v` to `/var/run/docker.sock`. Also, you need to mount the top-level directory (`/`) on the host machine in `ro` mode. Otherwise, Doku will not be able to calculate the size of the logs and bind mounts.

    docker run --name doku -d -v /var/run/docker.sock:/var/run/docker.sock:ro -v /:/hostroot:ro -p 9090:9090 amerkurev/doku

<br>

Doku will be available at [http://localhost:9090/](http://localhost:9090/).

# licence-anchor
## License

#### MIT
