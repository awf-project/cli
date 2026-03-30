---
title: "AWF Plugin improvements"
description: "AWF v0.5.0: Enhancing the Plugin Ecosystem."
date: 2026-03-30
draft: false
categories: ["announcements", "plugins"]
tags: ["awf", "release", "open-source", "go"]
contributors: ["Alex"]
---

## Key improvements in this release
Since the last release, I’ve been working on enhancing the plugin ecosystem, and I’m excited to announce that **v0.5.0** is now live!

Regarding the plugin architecture, AWF now leverages **go-plugin**, **gRPC gateway**, and **Protobuf**.
Plugins can now expose operations, commands (step-types), and validators. This move provides a more flexible architecture
and a significantly better user experience across the ecosystem.

```bash
Manage AWF plugins: list, enable, and disable plugins.

Plugins extend AWF functionality by providing custom operations,
commands, and validators.

Examples:
  awf plugin list
  awf plugin enable slack-notifier
  awf plugin disable slack-notifier

Usage:
  awf plugin [command]

Aliases:
  plugin, plugins

Available Commands:
  disable     Disable a plugin
  enable      Enable a plugin
  install     Install a plugin from GitHub releases
  list        List all available plugins
  remove      Remove an installed plugin
  search      Search for available plugins on GitHub
  update      Update an installed plugin to the latest version
```

Plugin examples are available in the [AWF repository](https://github.com/alexellis/awf/tree/main/examples). 
I also wrote a simple one, [time](https://github.com/awf-project/awf-plugin-time), specifically to help our beloved LLMs handle time. 
The documentation has been updated and can be found [here](https://awf-project.ai/cli/docs/user-guide/plugins/).

This release was important to me because, while plugins are not mandatory to use AWF, 
they are essential if you want to avoid writing overly long script files when interacting with other services.

I hope you enjoy this new feature!

## Other news
### Github action for workflows
I developed a [github action](https://github.com/awf-project/setup-awf) that allows you to run workflows directly
in your CI/CD pipelines. This is particularly useful for providing rich context to an agent when creating PRs in
other repositories (e.g., for documentation).

I’m not using this action myself just yet, but I plan to integrate it as soon as I merge the PR for the CLI-associated skills.

### Website
It took a whole week, but I finally caved and bought a domain! It’s just a [webpage](https://awf-project.ai/) for now, but it’s a start.

### Lightning talk
While attending [Symfony Live](https://live.symfony.com/2026-paris/), [Nicolas](https://github.com/nicolas-grekas)
convinced me to give a lightning talk about AWF. Here are the slides:

<iframe src="https://docs.google.com/presentation/d/e/2PACX-1vTXt9fyuQAvcC8N251lVbHoqMHjVA83WoTq8jFTffI0NFALWAz5lW9nBv4kzp4y7G0HybPkObzu4Rgx/pubembed?start=false&loop=false&delayms=3000" frameborder="0" width="960" height="569" allowfullscreen="true" mozallowfullscreen="true" webkitallowfullscreen="true"></iframe>

## Next version
The primary focus for the next release will be __workflows__. I want to refine this feature and enable workflow sharing.
This community-driven approach is a priority for me, as it will significantly improve the experience for everyone.

## Last but not least
I’ve been digging through my archives and found two other projects that I plan to integrate with the CLI.
The goal is to build a truly deterministic ecosystem for AWF.

I hope you’re ready to read some [Zig](https://ziglang.org/)!
