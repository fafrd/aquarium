# Bot Aquarium

This project gives a large language model (LLM) control of a Linux machine.

In the example below, we start with the prompt:

> You now have control of an Ubuntu Linux server. Your goal is to run a Minecraft server. Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.
> 
> Respond with a linux command to give to the server.

The AI first does a _sudo apt-get update_, then installs openjdk-8-jre-headless. Each time it runs a command we return the result of this command back to OpenAI and ask for a summary of what happened, then use this summary as part of the next prompt.

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

**arguments**

    ./aquarium -h
    Usage of ./aquarium:
      -context-mode string
        How much context from the previous command do we give the AI? This is used by the AI to determine what to run next.
        - partial: We send the last 10 lines of the terminal output to the AI. (cheap, accurate)
        - full: We send the entire terminal output to the AI. (expensive, very accurate)
         (default "partial")
      -debug
            Enable logging of AI prompts to debug.log
      -goal string
            Goal to give the AI. This will be injected within the following statement:

            > You now have control of an Ubuntu Linux server.
            > [YOUR GOAL WILL BE INSERTED HERE]
            > Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.
            >
            > Respond with a linux command to give to the server.
             (default "Your goal is to run a Minecraft server.")
      -limit int
            Maximum number of commands the AI should run. (default 30)
      -model string
            OpenAI model to use. gpt-4 is best, but most expensive. See https://platform.openai.com/docs/models (default "gpt-3.5-turbo")
      -preserve-container
            Persist docker container after program completes.
      -split-limit int
            When context-mode=full, we split up the response into chunks and ask the AI to summarize each chunk.
            split-limit is the maximum number of times we will split the response. (default 3)

## Logs

The left side of the screen contains general information about the state of the program. The right side contains the terminal, as seen by the AI.
<br />These are written to aquarium.log and terminal.log.

Calls to OpenAI are not logged unless you add the `--debug` flag. API requests and responses will be appended to debug.log.

# How it works

## Agent loop
1. Send the OpenAI api the list of commands (and their outcomes) executed so far, asking it what command should run next
1. Execute command in docker VM
1. Read output of previous command- send this to OpenAI and ask gpt-3.5-turbo for a summary of what happened
    1. If the output was too long, OpenAI api will return a 400
    1. Recursively break down the output into chunks, ask it for a summary of each chunk
    1. Ask OpenAI for a summary-of-summaries to get a final answer about what this command did

## more examples

Prompt: `Your goal is to execute a verbose port scan of amazon.com.`

The bot replies with _nmap -v amazon.com_. nmap is not installed; we return the failure to the AI, which then installs it and continues.

https://user-images.githubusercontent.com/5905628/227047932-1a87e7e7-43f9-48e0-aab2-bc83126b3be1.mp4

<br />

Prompt: `Your goal is to install a ngircd server.` (an IRC server software)

Installs the software, helpfully allows port 6667 through the firewall, then tries to run _sudo -i_ and gets stuck.

<img width="738" alt="Screenshot 2023-03-24 at 6 26 21 PM" src="https://user-images.githubusercontent.com/5905628/227677328-a8799002-bc93-4ee5-8f09-bfbc3461b46e.png">

# Todo

- There's no success criteria- the program doesn't know when to stop. The flag `-limit` controls how many commands are run (default 30)
- The AI cannot give input to running programs. For example, if you ask it to SSH into a server using a password, it will hang at the password prompt. For `apt-get`, i've hacked around this issue by injecting `-y` to prevent asking the user for input.
- I don't have a perfect way to detect when the command completes; right now I'm taking the # of running processes beforehand, running the command, then I poll the num procs until it returns back to the original value. This is a brittle solution
- The terminal output handling is imperfect. Some commands, like wget, use \\r to write the progress bar... I rewrite that as a \\n instead. I also don't have any support for terminal colors, which i'm suppressing with `ansi2txt`
