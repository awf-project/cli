---
title: "AWF Is Live"
description: "AWF is officially released — a retrospective on what it took to get here, what's working, and what's coming next."
date: 2026-03-21
draft: false
categories: ["announcements"]
tags: ["awf", "release", "open-source", "go"]
contributors: ["Alex"]
---

This has been a major week: AWF is now officially released.

## From "Building" to "Shipping"

Over the past two weeks, I focused on minor tweaks and bug fixes discovered while using devcontainers for my AI projects, rather than adding new features. 
I also wrote a [blog post (in French)](https://alex.balmes.co/fr/blog/l-ia-au-service-de-l-industrialisation-passer-de-30-de-reussite-a-100-de-fiabilite) about determinism in AI, which I plan to translate into English as soon as possible.

Writing that post made me realize something: it was time to stop saying "I'm building a tool" and actually show it.

So this week, I went through the documentation and double-checked every feature I don't use daily in my own workflows. 
It was a great exercise — it helped me identify non-functional features, which have now been removed to keep the codebase clean. What ships today is what actually works.

## State of the Agents

On the AI provider side, here's where things stand:

- **Claude** and **Gemini** — I'm using both heavily in my daily workflows. They are solid and well-tested.
- **Codex** — I don't personally use it, but I have many friends who love it.
- **OpenAI-compatible** — Not tested a lot but important for the future. It unlocks Ollama, vLLM, and any Chat Completions API for my local LLMs.
- **Mistral** — As a French developer, adding Mistral is a priority. It will be implemented ASAP.

## Upcoming work

I want to work and finish the "plugins" part of AWF, but I'll be at [SymfonyLive](https://live.symfony.com/2026-paris/) and need to focus on [my upcoming conference talk](https://grenoble2026.drupalcamp.fr/fr/events/grenoble/sessions/au-secours-me-demande-dutiliser-de-lia) first.

## Built by a Developer, for Developers

To be honest, I'm curious to see how people will use AWF. It's a tool built by a developer, for developers. I truly hope you find it useful.

But even if it doesn't find a wide audience, I'll continue adding features — because I use it every day, and that's what matters most.

And if you do like it... well, I have plenty of ideas for what's next.
