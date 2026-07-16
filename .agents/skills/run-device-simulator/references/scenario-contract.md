# Deterministic scenario contract

Every future scenario declares:

- synthetic device identity and advertised API version;
- secure or explicitly insecure test transport;
- entity descriptors and initial states;
- virtual-time state events;
- expected client commands, matching rules, and deadlines;
- named protocol/network faults and their exact trigger;
- seed for any generated load data;
- expected disconnect and cleanup state;
- support-matrix row and evidence level it exercises.

Scenarios must be runnable in-process and parallel without fixed external ports. Real time is allowed only at the outer test deadline. Assertions must fail with sanitized protocol context and must not print keys or network identities.
