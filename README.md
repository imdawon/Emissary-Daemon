# Emissary-Daemon (Desktop Platform)
![Emissary Logo](./emissary_logo.jpg)

[Click here to quickly set up Drawbridge and Emissary](https://github.com/dhens/Drawbridge/wiki/Quick-Start-Up-Guide-%E2%80%90-Get-Drawbridge-and-Emissary-protecting-your-applications-%E2%80%90-v0.1.0%E2%80%90alpha)

**Note: Emissary for Desktops should not be downloaded directly. It should only be downloaded using the Emissary Bundle feature in the [Drawbridge reverse proxy](https://github.com/dhens/Drawbridge).**

## Usage
`./Emissary`

### Arguments
```
--outbound <host:port>: Enable Emissary Outbound Mode to host a locally accessible service e.g a Minecraft Server via Drawbridge.
--service-name <name>: Outbound service name as seen in Emissary and the Drawbridge Dashboard.
```


A desktop agent used to prove a set of conditions required by the [Drawbridge reverse proxy](https://github.com/dhens/Drawbridge) to be granted an mTLS certificate to access resources beyond Drawbridge. 

Self-hosting is a nightmare. If you're naive, you blow a hole in your home router to allow access to whatever resource you want to have accessible via the internet. If you're *"smart"*, you let some other service handle the ingress for you, most likely allowing for traffic inspection and mad metadata slurp-age by said service. Even if there's none of that, it doesn't really feel like you're sticking it to the man when you have to rely on a service to keep your self-hosted applications secure.

Emissary and Drawbridge solve this problem. Add Emissary to as many of your machines as you want, expose the Drawbridge reverse proxy server with required authentication details, _instead_ of your insecure web application or "movie server", and bam: your service is only accessible from approved Emissary clients.

Emissary does this by connecting to the Drawbridge server, which the user specifies in the Emissary app. Drawbridge will respond with a list of required configuration details for the machine running Emissary, such as being an updated Windows 11 machine, matching a specific serial number, connecting from a specific IP range. 

Drawbridge will routinely check in on Emissary clients to ensure they continue to match the required configuration. If an Emissary client fails to meet the Drawbridge Policy standards, Drawbridge will revoke the mTLS certificate, shutting off access to your resources beyond it. 

[Click here to read more about how Drawbridge works](https://github.com/dhens/Drawbridge).

## Features

### Emissary Outbound: 
**Deploy your Protected Services that live “behind enemy lines”.**

Punch a hole outward with Emissary Outbound to create a Protected Service where you don’t control the network / can’t allow ingress from the internet.

Emissary Outbound creates a tunnel between a service it can access and Drawbridge, effectively making Emissary a mini-Drawbridge.

Your machines can now securely proxy out connections to local services to Drawbridge as a Protected Service.

How to use

Emissary Outbound is now a feature of the Emissary-Daemon client. It is enabled via passing two fields in the command line when launching Emissary:
--outbound is the host and port for the service you want to allow access to via Drawbridge
--service-name MUST be 15 characters in length with no special characters due to a bug. This will be fixed in the non-preview version.

Note: With this update, you will not see this Protected Service in the Drawbridge Dashboard yet, but you will in any connecting Emissary clients.

Example for a Minecraft server (note you'll need to change the address and port if using a different computer or port):
./Emissary_program --outbound localhost:25565 --service-name MinecraftServer

Now, when a regular Emissary client connections to your Outbound-Protected service, the connection flow will look like this:
Emissary <-> Drawbridge <-> Emissary Outbound <-> Outbound-Protected service (e.g Minecraft server)


