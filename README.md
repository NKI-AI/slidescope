# SlideScope

Backend to generate a DeepZoom pyramid based on [OpenSlide Go]. Compatible with [OpenSeaDragon] and [OpenLayers].

[OpenSlide Go]: https://github.com/NKI-AI/openslide-go.git
[OpenSeaDragon]: https://openseadragon.github.io/
[OpenLayers]: https://openlayers.org/

## Features

- Read any openslide object, including overlay masks and convert these into a pyramid
- RESTful API to add images/overlays
- Logging in with JWT token

## Not-yet Features

- Add data through API key
- Associate images with users
- Login does not have an effect yet