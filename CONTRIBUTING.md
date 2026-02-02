# Contributing to Control Plane

:+1::tada: First off, thanks for taking the time to contribute! :tada::+1:

Control Plane use the Apache 2.0 licence and accepts contributions via GitHub pull requests.

The following is a set of guidelines for contributing to a Control Plane project. We generally have stricter rules as we focus on security but don't let that discourage you from creating your PR, it can be incrementally fixed to fit the rules. Also feel free to propose changes to this document in a pull request.

## Table Of Contents

- [Contributing to Control Plane](#contributing-to-control-plane)
  - [Table Of Contents](#table-of-contents)
  - [I Don't Want To Read This Whole Thing I Just Have a Question!!!](#i-dont-want-to-read-this-whole-thing-i-just-have-a-question)
  - [How Can I Contribute?](#how-can-i-contribute)
    - [Reporting Bugs](#reporting-bugs)
      - [Before Submitting a Bug Report](#before-submitting-a-bug-report)
      - [How Do I Submit a (Good) Bug Report?](#how-do-i-submit-a-good-bug-report)
    - [Suggesting Enhancements](#suggesting-enhancements)
      - [Before Submitting an Enhancement Suggestion](#before-submitting-an-enhancement-suggestion)
      - [How Do I Submit A (Good) Enhancement Suggestion?](#how-do-i-submit-a-good-enhancement-suggestion)
    - [Your First Code Contribution](#your-first-code-contribution)
      - [Development](#development)
    - [Pull Requests](#pull-requests)
  - [Style Guides](#style-guides)
    - [Git Commit Messages](#git-commit-messages)

---

## I Don't Want To Read This Whole Thing I Just Have a Question!!!

Please message the Zulip `cp-training` stream. We also have an issue template for questions.

## How Can I Contribute?

### Reporting Bugs

This section guides you through submitting a bug report. Following these guidelines helps maintainers understand your report, reproduce the behaviour, and find related reports.

Before creating bug reports, please check [this list](#before-submitting-a-bug-report) as you might find out that you don't need to create one. When you are creating a bug report, please [include as many details as possible](#how-do-i-submit-a-good-bug-report). Fill out the issue template for bugs, the information it asks for helps us resolve issues faster.

> **Note:** If you find a **Closed** issue that seems like it is the same thing that you're experiencing, open a new issue
> and include a link to the original issue in the body of your new one.
#### Before Submitting a Bug Report

- **Perform a cursory search** to see if the problem has already been reported. If it has **and the issue is still open**, add a comment to the existing issue instead of opening a new one

#### How Do I Submit a (Good) Bug Report?

Bugs are tracked as [GitHub issues](https://guides.github.com/features/issues/).
Create an issue on within the repository and provide the following information by filling in the issue template.

Explain the problem and include additional details to help maintainers reproduce the problem:

- **Use a clear and descriptive title** for the issue to identify the problem
- **Describe the exact steps which reproduce the problem** in as many details as possible.
- **Describe the behaviour you observed after following the steps** and point out what exactly is the problem with that behaviour
- **Explain which behaviour you expected to see instead and why.**

Provide more context by answering these questions:

- **Did the problem start happening recently** or was this always a problem?
- If the problem started happening recently, **can you reproduce the problem in an older version/release of the Training Infrastructure?** What's the most recent version in which the problem doesn't happen?
- **Can you reliably reproduce the issue?** If not, provide details about how often the problem happens and under which conditions it normally happens

### Suggesting Enhancements

This section guides you through submitting an enhancement suggestion for Training Infrastructure, including completely new features and minor improvements to existing functionality. Following these guidelines helps maintainers understand your suggestion and find related suggestions.

Before creating enhancement suggestions, please check [this list](#before-submitting-an-enhancement-suggestion) as you might find out that you don't need to create one. When you are creating an enhancement suggestion, please [include as many details as possible](#how-do-i-submit-a-good-enhancement-suggestion). Fill in the template feature request template, including the steps that you imagine you would take if the feature you're requesting existed.

#### Before Submitting an Enhancement Suggestion

- **Check if there's already a covering enhancement**
- **Perform a cursory search** to see if the enhancement has
  already been suggested. If it has, add a comment to the existing issue instead of opening a new one

#### How Do I Submit A (Good) Enhancement Suggestion?

Enhancement suggestions are tracked as [GitHub issues](https://guides.github.com/features/issues/). Make sure to provide the following information:

- **Use a clear and descriptive title** for the issue to identify the suggestion
- **Provide a step-by-step description of the suggested enhancement** in as many details as possible
- **Provide specific examples to demonstrate the steps**. Include copy/pasteable snippets which you use in those examples,
  as [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines)
- **Describe the current behaviour** and **explain which behaviour you expected to see instead** and why
- **Explain why this enhancement would be useful** to most Kubesec users and isn't something that can or should be implemented
  as a separate community project
- **List some other tools where this enhancement exists.**
- **Specify which version of Kubesec you're using.** You can get the exact version by running `kubesec version` in your terminal
- **Specify the name and version of the OS you're using.**

### Your First Code Contribution

Unsure where to begin contributing to the Training infrastructure? You can start by looking through these `Good First Issue` and `Help Wanted`
issues:

- [Good First Issue issues][good_first_issue] - issues which should only require a few lines of code, and a test or two
- [Help wanted issues][help_wanted] - issues which should be a bit more involved than `Good First Issue` issues

Both issue lists are sorted by total number of comments. While not perfect, number of comments is a reasonable proxy for impact a given change will have.

#### Development

- TBC

### Pull Requests

The process described here has several goals:

- Maintain training quality
- Fix problems that are important to the training team and users
- Enable a sustainable system for training maintainers to review contributions

Please follow these steps to have your contribution considered by the maintainers:

1. Follow all instructions in the template
2. Follow the [style guides](#style-guides)
3. After you submit your pull request, verify that all [status checks](https://help.github.com/articles/about-status-checks/)
   are passing
   <details>
    <summary>What if the status checks are failing?</summary>
    If a status check is failing, and you believe that the failure is unrelated to your change, please leave a comment on
    the pull request explaining why you believe the failure is unrelated. A maintainer will re-run the status check for
    you. If we conclude that the failure was a false positive, then we will open an issue to track that problem with our
    status check suite.
   </details>


While the prerequisites above must be satisfied prior to having your pull request reviewed, the reviewer(s) may ask you to complete additional tests, or other changes before your pull request can be ultimately accepted.

## Style Guides

### Git Commit Messages

- It's strongly preferred you [GPG Verify][commit_signing] your commits if you can
- Follow [Conventional Commits](https://www.conventionalcommits.org)
- Use the present tense ("add feature" not "added feature")
- Use the imperative mood ("move cursor to..." not "moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line
