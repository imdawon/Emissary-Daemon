How does Emissary know what service I am trying to connect to?

Emissary will expose a local list of ports for each Protected Service Drawbridge will allow it to access. It starts at port 3100 and increments by 1 for each new Protected Service. 
3100 has been chosen as the starting port due to its generic use and low chance of collision with other service ports.
When Emissary connects to Drawbridge, Drawbridge sends down a list of each service that can be accessed. Emissary then assigns each service to a port on the host machine.
Then, when Emissary detects traffic on a port it has exposed, it will send the name of the service to Drawbridge and attempt to connect. If the Emissary client has been configured to access the resource, Drawbridge will lower the gate and allow traffic to flow. Otherwise, the traffic is dropped. 