---
title: "New release! v0.6.0 brings workflow packs to the AWF ecosystem"
description: "AWF v0.6.0 is here! Introducing workflow packs for easier sharing, a new focus on AI agents, and a look at why AWF is becoming the CI/CD for agents."
date: 2026-04-05
draft: false
categories: ["announcements"]
tags: ["awf", "cli", "ai", "workflow"]
contributors: ["Alex"]
---

Last week was a blast! I had many great discussions about AWF and AI in general at Symfony Live. 
These conversations convinced me that the AWF ecosystem definitely needs a way to share workflows. 
Although I’m still exploring the best approach, these "packs" will hopefully help spread the word about AWF.

That’s why I’ve been working on [__workflow packs__](https://awf-project.ai/cli/docs/user-guide/workflow-packs/), building on the plugin concept. 
A workflow pack is a set of workflows, prompts, and scripts that can be installed like a plugin via dedicated commands.
The only thing you need to be mindful of is what these workflows actually do on your machine,
given the security implications of AWF’s execution.

I also experimented with [translating SpecKit into an AWF workflow](https://github.com/awf-project/awf-workflow-speckit). 
This journey gave me several ideas on how to improve the agent component. 
Consequently, __agents will be the main focus of the next release__.

## Why should you use AWF

I want to clarify my position on this, especially since my recent conversations were so insightful.

I believe AWF is important because AI should give us more time to think about how to improve our daily tasks and our work.
Like me, you’ve probably wasted a lot of time writing scripts to optimize a 5-minute task—and if you love coding as much as I do,
"wasting" that day felt fantastic. AI is a powerful tool because it allows us to create and maintain many of these "small things".
In my opinion, building a workflow is the same thing ([with a dedicated skill](https://github.com/awf-project/awf-marketplace/) if you want effective assistance).

My goal for AWF is simple: __AWF should be the CI/CD for your agents__. 
The more you use AI, the more you need tools to avoid divergence and issues: tests, linters, TDD, architectural violations, etc.

AWF allows you to run these tools—your tools—in a deterministic way,
stopping the process if an agent makes a poor decision. 
With AWF, you aren't just sitting behind your screen hitting "Enter" and waiting for the next prompt.
Using AWF is a quality constraint for your brain: __think about your feature, then build it with your workflow__.

Workflows bring peace of mind because, once created, they are consistent.
You design them according to your own standards. Same execution, same CLI tools.
This is more than automation, __this is industrialization__. If you're tired or having a bad day, your workflows will work as usual.
If you're stuck in meetings and short on time, AWF will run your workflow as usual.
__To me, that sounds like progress__.

## On another note

I’ll be giving a talk about AI once a week for the next three weeks! 
You can catch me at [DrupalCamp](https://grenoble2026.drupalcamp.fr/fr/events/grenoble/sessions/au-secours-me-demande-dutiliser-de-lia),
then at [AFUP Lyon](https://www.meetup.com/fr-fr/afup-lyon-php/events/313790803/?eventOrigin=group_upcoming_events), and finally at [Dev with AI (tba)](https://luma.com/devwithai).
I’m particularly looking forward to the last one; it’s going to be fun because I know many people at the host company.
We used to code in PHP together, but they’ve since moved to a GoLang... and as it happens, AWF is written in GoLang!
