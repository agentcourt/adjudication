> *We are slaves of the laws in order that we may be free.*
> 
> --Cicero

## Summary

This project is an experimental AI civil litigation system that uses
agent attorneys with either agent or human clients.

The technical approach is a little unusual.  The implementation:

1. Uses a core procedural engine implemented in
   [Lean](https://lean-lang.org/) with [many
   theorems](proofstats.md) about its behavior.

1. Supports verifiable execution in [attestable instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nitrotpm-attestation.html) and Trusted
   Execution Environments, which also provide confidentiality.
   
1. Roughly follows the [United States Federal Rules for Civil
   Procedure](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure)
   (FRCP).  Our version of the rules is called the [Agent Rules
   for Civil Procedure](ARCP.md) (ARCP).

1. Interacts with agent attorneys via an implementation of the [Agent
   Client
   Protocol](https://agentclientprotocol.com/get-started/introduction)
   extended to support external tool calls for litigation.  This
   approach facilitates arbitrary computer use by attorney-agent teams.
   
1. Provides somewhat sophisticated [sampling for candidate pools of AI
   jurors](juries.md).  (Attorneys still have access to *voir dire*
   under [ARCP](ARCP.md) [Rule
   47](ARCP.md#rule-47-selecting-jurors).)

We are starting to use this system as a simulator to use in developing
agent attorneys and judges.


## Design

A Go layer talks to agent attorneys via an extension of the Agent
Client Protocol.  This Go layer also interacts with the core
procedures engine, which is implemented in Lean.  This Lean engine is
responsible for executing [ARCP](ARCP.md) correctly, and the
code includes many theorems about its behavior.

The entire litigation process can run in an enclave for attestable and
optionally confidential execution.  Agent attorneys can run either
inside that enclave or remotely.

## Directions

This experiment is evolving. Here are some of the main directions:

1. **More and more meaningful theorems** about the behavior of the
   core procedures engine.  Ideally these theorems cover all of ARCP
   as well as a lot of critical specifications that are not explicitly
   stated in the rules.  For example, one category of theorems relates
   to the many parameterized litigation limits that should be enforced
   at each turn, during certain phases, and overall.
   
1. **Migration of some runtime code from Go to Lean.**  Currently a
   clean, logical boundary between Go and Lean is important and very
   valuable, so we are reluctant to mess with that too much.  Instead,
   we will probably introduce a more runtime-oriented Lean layer, and
   some functionality currently implemented in Go can move there
   incrementally.
   
1. **More scenario-based testing.**  We have some pretty good gear for
   specifying scenarios, and we'd like to build on that more while
   also increasing its expressiveness.
   
1. **Agent law firms**: We have started work on making good agent
   attorneys.  We can do more, and others presumably can, too.  Some
   possibilities: Mock trials, expert-like technical analysis,
   knowledge of the rules, *voir dire* techniques, etc.  The current
   system is suitable (but currently far too slow and expensive) as a
   simulator that can be used for reinforcement learning.
   
1. **Better judges**: Currently our AI judges have basic grounding,
   and they have a strong bias in favor of cases going to a jury
   trial.  Judges have some discretion, but the [rules](ARCP.md) are
   of course strictly enforced. It's easy to imagine more sophisticated
   approaches for judge behavior, and we are beginning to explore
   these opportunities.  Again the current system is a suitable (but
   currently far too slow and expensive) as a simulator that can be used
   for reinforcement learning.  We are also connecting indexed corpora
   to assist judges.  Parties will be able to choose which courts to
   use (and those courts can have their own local limits and court
   policy, as described in the [Local Rules Limits
   Guide](limits.md)).

1. **Automated iterative system improvement**: We have
   [rules](ARCP.md) written in English, and we can [prove
   theorems](proofstats.md).  We also have broader and [richer
   rules](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure)
   as a guide.  We also have lots of case law.  Can we set up
   processes that improve the system automatically?

1. Actual dispute resolution!


## References

1. [United States Federal Rules of Civil Procedure](https://www.uscourts.gov/rules-policies/current-rules-practice-procedure/federal-rules-civil-procedure) (FRCP)
1. [Agent Rules for Civil Procedure](ARCP.md) (ARCP)
1. [Lean](https://lean-lang.org/)
1. [Agent Client Protocol](https://agentclientprotocol.com/get-started/introduction) (ACP)
1. [AWS EC2 instance attestation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nitrotpm-attestation.html)
