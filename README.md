# bot aquarium

imagine xkcd/350, but with ai:

[![https://xkcd.com/350/](https://imgs.xkcd.com/comics/network.png)](https://xkcd.com/350/)
k


## Build docker image

    docker network create aquarium
    docker build -t instance .

## start

    OPENAI_API_KEY=your_key_here go run aquarium
