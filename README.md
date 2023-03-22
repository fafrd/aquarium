# bot aquarium

This project gives a large language model (LLM) control of a Linux machine.

In the example below, we use the prompt:

> You now have control of an Ubuntu Linux server. Your goal is to execute a verbose port scan of amazon.com. Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.
> 
> Respond with a linux command to give to the server.

The bot replies with _nmap -v amazon.com_. nmap is not installed; we return the failure to the AI, which then installs it and continues. 

https://user-images.githubusercontent.com/5905628/227047932-1a87e7e7-43f9-48e0-aab2-bc83126b3be1.mp4

<br />Inspired by [xkcd.com/350](https://xkcd.com/350/) and [Optimality is the tiger, agents are its teeth](https://www.lesswrong.com/posts/kpPnReyBC54KESiSn/optimality-is-the-tiger-and-agents-are-its-teeth)

## Build docker image

    docker network create aquarium
    docker build -t instance .

## start

    OPENAI_API_KEY=your_key_here go run aquarium
