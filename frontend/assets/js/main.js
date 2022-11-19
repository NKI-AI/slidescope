import '../css/style.css';
import TileLayer from 'ol/layer/Tile';
import {Map, View} from "ol";
import {Zoomify} from "ol/source";
import {ScaleLine, defaults as defaultControls} from 'ol/control';


function loadUrl ( url,
                   opt_options  // attributions (defaults to undefined), crossOrigin (defaults to 'anonymous')
) {

    let options = opt_options || {};
    let crossOrigin = options.crossOrigin === undefined ? 'anonymous' : options.crossOrigin;

    let layer = new TileLayer({});

    let last = url.lastIndexOf('.');
    let path = url.slice(0, last);

    let xhr = new XMLHttpRequest();
    xhr.open('GET', url);
    xhr.onload = function() {

        let parser = new DOMParser();
        let xmlDoc = parser.parseFromString(xhr.responseText,'text/xml');

        let elements = xmlDoc.getElementsByTagName('Image');
        let tileSize = Number(elements[0].getAttribute('TileSize'));
        let format = elements[0].getAttribute('Format');
        let width = Number(elements[0].getElementsByTagName('Size')[0].getAttribute('Width'));
        let height = Number(elements[0].getElementsByTagName('Size')[0].getAttribute('Height'));
        let url = path + '_files/{z}/{x}_{y}.' + format;

        let source = new Zoomify({
            attributions: options.attributions,
            url: url,
            size: [width, height],
            tileSize: tileSize,
            crossOrigin: crossOrigin
        });

        const offset = Math.ceil(Math.log(tileSize) / Math.LN2);

        source.setTileUrlFunction(function (tileCoord) {
            return url.replace(
                '{z}', tileCoord[0] + offset
            ).replace(
                '{x}', tileCoord[1]
            ).replace(
                '{y}', tileCoord[2]
            );
        });

        layer.setExtent([0, -height, width, 0]);
        layer.setSource(source);

    }
    xhr.send();
    return layer;
}

function scaleControl() {
    let control;
    control = new ScaleLine({
        units: "metric",
    });
    return control;
}

let map = new Map({
    controls: defaultControls().extend([scaleControl()]),
    target: 'map',
    logo: false
});


let searchParams = new URLSearchParams(window.location.search)
let dziUrl

if (searchParams.has('id') === true) {
    // Add the image itself
    let imageId = searchParams.get('id')
    dziUrl = 'deepzoom/' + imageId + '/slide.dzi'
    console.log("Loading deepzoom %s", dziUrl)

    let layer = loadUrl(
        dziUrl,
        { attributions: '&copy 2022, <a href="https://aiforoncology.nl/" target="_blank">AI for Oncology</a>' }
    );

    layer.on('change:source', function(evt) {
        map.setView(
            new View({
                resolutions: layer.getSource().getTileGrid().getResolutions(),
                extent: layer.getExtent(),
                constrainOnlyCenter: true
            })
        );
        map.getView().fit(layer.getExtent(), { size: map.getSize() });
    });

    map.addLayer(layer);
}


