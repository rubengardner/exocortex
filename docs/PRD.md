Project: Exocortex
Concept: A terminal-centric command center designed to unify the software development lifecycle. By integrating Jira, GitHub, and local development tools into a single CLI interface, Exocortex eliminates context switching and leverages Git Worktrees for seamless multitasking.

1. Core Nomenclature
   To maintain the biological theme of the Exocortex, the system uses the following hierarchy:

Nucleus: The primary organizational unit associated with a specific task, Jira ticket or Github PR (Or both JIRA and GITHUB or none of them). It acts as the "command hub" for a unique branch and worktree.

Neuron (formerly Agent): Specialized execution instances within a Nucleus. These are terminal-based tools-such as ClaudeCode instances, Neovim sessions, or shell processes-dedicated to solving parts of the task.

2. The Nucleus Interface (CLI UX)
   The interface is designed to provide a holistic view of a task without requiring a browser or a separate IDE.

The Nucleus Sidebar: A persistent left-hand menu that allows users to switch between active Nuclei. Selecting a Nucleus updates the main view to reflect the specific context of that task.

The Holistic Dashboard: Inside a Nucleus, the screen is divided into relevant functional windows:

Ticket Insight: A metadata panel showing the Jira ticket description and status, with a quick-link to the web view.

Neuron Cluster: Multiple terminal panes where AI-powered Neurons (like ClaudeCode) assist with logic and automation.

Editor Pane: A dedicated space to launch Neovim directly into the Nucleus-specific worktree.

Contextual Data: Real-time information on branch status, PR statistics, and environment variables.

3. Operations & Workflows
   A Nucleus is generated through one of two primary workflows, both powered by automated Git Worktree management.

A. The Development Workflow
Trigger: Creating a Nucleus from a Jira ticket on the integrated Board View.

Automation: The system automatically initializes a new Git Worktree and creates a branch following the standard: task/<jira_ticket_number>/ and we manually name the rest of the branch.

Objective: To allow the developer to start coding in a clean, isolated environment without disturbing other active tasks.

B. The Review Workflow
Trigger: Selecting an existing PR or a ticket in the "Review" column.

Automation: Instead of creating a new branch, the system allows the user to search/filter for an existing branch from the repository and checks it out into a dedicated worktree.

Review Insights: The Nucleus displays a GitHub PR Summary, including:

Classic Stats: Lines added/removed and total files changed.

File Browser: A list of changed files with the ability to preview diffs.

Deep-Linking: A shortcut to open any changed file in Neovim at the exact location of the modifications.

4. Integration Views
   Exocortex provides top-level views to manage work before it is assigned to a Nucleus.

The Board View (Jira): A terminal-based representation of your Jira board. Users can select a ticket and hit a shortcut to "Spawn Nucleus," which handles all the backend setup.

The GitHub View: A high-level dashboard displaying all open Pull Requests (both personal and teammate-owned).

This view allows for "Ad-hoc Nuclei"-workspaces created for tasks that aren't officially tracked in Jira (e.g., quick feedback sessions or minor hotfixes
