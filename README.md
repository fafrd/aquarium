# Bot Aquarium

This project gives a large language model (LLM) control of a Linux machine.

In the example below, we start with the prompt:

> You now have control of an Ubuntu Linux server. Your goal is to run a Minecraft server. Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.
> 
> Respond with a linux command to give to the server.

The AI first has does a _sudo apt-get update_, then installs openjdk-8-jre-headless. Each time it runs a command we return the result of this command back to OpenAI and ask for a summary of what happened, then use this summary as part of the next prompt.

[![asciicast](https://asciinema.org/a/0CH4ESDjt4H11WABiMlGZNMYU.png?)](https://asciinema.org/a/0CH4ESDjt4H11WABiMlGZNMYU?&speed=2&i=2&autoplay=1)

Inspired by [xkcd.com/350](https://xkcd.com/350/) and [Optimality is the tiger, agents are its teeth](https://www.lesswrong.com/posts/kpPnReyBC54KESiSn/optimality-is-the-tiger-and-agents-are-its-teeth)

# Usage

## Build

    docker network create aquarium
    docker build -t aquarium .
    go build

## Start

Pass your prompt in the form of a goal. For example, `--goal "Your goal is to run a minecraft server."`

    OPENAI_API_KEY=$OPENAI_API_KEY ./aquarium --goal "Your goal is to run a Minecraft server."

## Logs

The left side of the screen contains general information about the state of the program. The right side contains the terminal, as seen by the AI.
<br />These are written to aquarium.log and terminal.log.

Calls to OpenAI are not logged unless you add the `--debug` flag. API requests and responses will be appended to debug.log.

# How it works

## Agent loop
1. Send the OpenAI api the list of commands (and their outcomes) executed so far, asking it what command should run next
1. Execute command in docker VM
1. Read output of previous command- send this to OpenAI and ask text-davinci-003 for a summary of what happened
    1. If the output was too long, OpenAI api will return a 400
    1. Recursively break down the output into chunks, ask it for a summary of each chunk
    1. Ask OpenAI for a summary-of-summaries to get a final answer about what this command did

## Another example

Prompt: `Your goal is to execute a verbose port scan of amazon.com.`

The bot replies with _nmap -v amazon.com_. nmap is not installed; we return the failure to the AI, which then installs it and continues.

https://user-images.githubusercontent.com/5905628/227047932-1a87e7e7-43f9-48e0-aab2-bc83126b3be1.mp4
