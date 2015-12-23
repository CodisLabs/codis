/* eslint indent: [2, 4] */
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = factory;
    } else {
        factory(Highcharts);
    }
}(function (Highcharts) {
    Highcharts.extend(Highcharts.SVGElement.prototype, {
        init: function (renderer, nodeName) {
            this.element = {
                nodeName: nodeName,
                attributes: {},
                childNodes: [],
                setAttribute: function (key, value) {
                    this.attributes[key] = value;
                },
                removeAttribute: function (key) {
                    delete this.attributes[key];
                },
                appendChild: function (element) {
                    this.childNodes.push(element);
                    element.parentNode = this;
                },
                insertBefore: function (newItem, existingItem) {
                    this.childNodes.splice(this.childNodes.indexOf(existingItem), 0, newItem);
                },
                removeChild: function (element) {
                    this.childNodes.splice(this.childNodes.indexOf(element), 1);
                    delete element.parentNode;
                },
                getElementsByTagName: function () {
                    return [];
                },
                cloneNode: function () {
                    return this;
                },
                style: {
                }
            };
            this.renderer = renderer;
            this.dSetter = function (value) {
                value.join = false; // don't join
                return value;
            };
        },
        getBBox: function () {
            var ctx = this.renderer.ctx,
                bBox;
            ctx.font = '12px Arial';
            ctx.fillStyle = 'blue';
            bBox = ctx.measureText(this.element.innerHTML);
            return { x: 0, y: 0, width: bBox.width, height: 20 };
        }
    });


    /**
     * Singleton holding drawing methods for each SVG shape type
     */
    var draw = {
        path: function (elem, ctx, attr) {
            var arr = attr.d,
                segments = [];
            arr.forEach(function (item) {
                if (item === 'M') {
                    segments.push(['moveTo']);
                } else if (item === 'L') {
                    segments.push(['lineTo']);
                } else if (item === 'C') {
                    segments.push(['bezierCurveTo']);
                } else if (item === 'Z') {
                    segments.push(['closePath']);
                } else {
                    segments[segments.length - 1].push(parseFloat(item));
                }
            });
            ctx.beginPath();
                    
            segments.forEach(function (segment) {
                ctx[segment[0]].apply(ctx, segment.slice(1));
            });
        },
        rect: function (elem, ctx, attr) {
            ctx.rect(attr.x, attr.y, attr.width, attr.height);
        },
        text: function (elem, ctx, attr) {
            if (elem.innerHTML) {
                ctx.font = '12px Arial';
                ctx.fillStyle = 'blue';
                elem.bBox = ctx.measureText(elem.innerHTML);
                ctx.fillText(elem.innerHTML, attr.x, attr.y);
            }
        }
    };
    Highcharts.extend(Highcharts.SVGRenderer.prototype, {
        init: function (container, width, height) {
            var renderer = this,
                boxWrapper,
                element;

            boxWrapper = renderer.createElement('svg')
                .attr({
                    version: '1.1'
                });
            element = boxWrapper.element;

            
            this.canvas = document.createElement('canvas');
            container.appendChild(this.canvas);
            this.ctx = this.canvas.getContext('2d');

            
            // object properties
            renderer.isSVG = true;
            renderer.box = element;
            renderer.boxWrapper = boxWrapper;
            renderer.alignedObjects = [];

            renderer.defs = this.createElement('defs').add();
            renderer.gradients = {}; // Object where gradient SvgElements are stored
            renderer.cache = {}; // Cache for numerical bounding boxes

            renderer.setSize(width, height, false);
        },
        setSize: function (width, height) {
            this.canvas.setAttribute('width', width);
            this.canvas.setAttribute('height', height);
        },
        draw: function () {
            var ctx = this.ctx;
            function drawElement(elem) {

                // Draw child nodes
                Array.prototype.forEach.call(elem.childNodes, drawElement);

                var attr = elem.attributes;
                if (attr) {


                    (draw[elem.nodeName] || function () {})(elem, ctx, attr);

                    if (attr['stroke-width']) {
                        ctx.lineWidth = attr['stroke-width'];
                        ctx.strokeStyle = attr.stroke;
                        ctx.stroke();
                    }
                    if (attr.fill) {
                        ctx.fillStyle = attr.fill;
                        ctx.fill();
                    }
                }
                
            }
            drawElement(this.box);
        },
        buildText: function (wrapper) {
            wrapper.element.innerHTML = wrapper.textStr;
        }
    });
}));
