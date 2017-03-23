/**
 * @license Highmaps JS v1.1.10 (2015-12-07)
 * Highmaps as a plugin for Highcharts 4.1.x or Highstock 2.1.x (x being the patch version of this file)
 *
 * (c) 2011-2014 Torstein Honsi
 *
 * License: www.highcharts.com/license
 */
/* eslint indent: [2, 4] */
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = factory;
    } else {
        factory(Highcharts);
    }
}(function (Highcharts) {


    var UNDEFINED,
        Axis = Highcharts.Axis,
        Chart = Highcharts.Chart,
        Color = Highcharts.Color,
        Point = Highcharts.Point,
        Pointer = Highcharts.Pointer,
        Legend = Highcharts.Legend,
        LegendSymbolMixin = Highcharts.LegendSymbolMixin,
        Renderer = Highcharts.Renderer,
        Series = Highcharts.Series,
        SVGRenderer = Highcharts.SVGRenderer,
        VMLRenderer = Highcharts.VMLRenderer,

        addEvent = Highcharts.addEvent,
        each = Highcharts.each,
        error = Highcharts.error,
        extend = Highcharts.extend,
        extendClass = Highcharts.extendClass,
        merge = Highcharts.merge,
        pick = Highcharts.pick,
        defaultOptions = Highcharts.getOptions(),
        seriesTypes = Highcharts.seriesTypes,
        defaultPlotOptions = defaultOptions.plotOptions,
        wrap = Highcharts.wrap,
        noop = function () {};

        /**
     * Override to use the extreme coordinates from the SVG shape, not the
     * data values
     */
    wrap(Axis.prototype, 'getSeriesExtremes', function (proceed) {
        var isXAxis = this.isXAxis,
            dataMin,
            dataMax,
            xData = [],
            useMapGeometry;

        // Remove the xData array and cache it locally so that the proceed method doesn't use it
        if (isXAxis) {
            each(this.series, function (series, i) {
                if (series.useMapGeometry) {
                    xData[i] = series.xData;
                    series.xData = [];
                }
            });
        }

        // Call base to reach normal cartesian series (like mappoint)
        proceed.call(this);

        // Run extremes logic for map and mapline
        if (isXAxis) {
            dataMin = pick(this.dataMin, Number.MAX_VALUE);
            dataMax = pick(this.dataMax, -Number.MAX_VALUE);
            each(this.series, function (series, i) {
                if (series.useMapGeometry) {
                    dataMin = Math.min(dataMin, pick(series.minX, dataMin));
                    dataMax = Math.max(dataMax, pick(series.maxX, dataMin));
                    series.xData = xData[i]; // Reset xData array
                    useMapGeometry = true;
                }
            });
            if (useMapGeometry) {
                this.dataMin = dataMin;
                this.dataMax = dataMax;
            }
        }
    });

    /**
     * Override axis translation to make sure the aspect ratio is always kept
     */
    wrap(Axis.prototype, 'setAxisTranslation', function (proceed) {
        var chart = this.chart,
            mapRatio,
            plotRatio = chart.plotWidth / chart.plotHeight,
            adjustedAxisLength,
            xAxis = chart.xAxis[0],
            padAxis,
            fixTo,
            fixDiff,
            preserveAspectRatio;


        // Run the parent method
        proceed.call(this);

        // Check for map-like series
        if (this.coll === 'yAxis' && xAxis.transA !== UNDEFINED) {
            each(this.series, function (series) {
                if (series.preserveAspectRatio) {
                    preserveAspectRatio = true;
                }
            });
        }

        // On Y axis, handle both
        if (preserveAspectRatio) {

            // Use the same translation for both axes
            this.transA = xAxis.transA = Math.min(this.transA, xAxis.transA);

            mapRatio = plotRatio / ((xAxis.max - xAxis.min) / (this.max - this.min));

            // What axis to pad to put the map in the middle
            padAxis = mapRatio < 1 ? this : xAxis;

            // Pad it
            adjustedAxisLength = (padAxis.max - padAxis.min) * padAxis.transA;
            padAxis.pixelPadding = padAxis.len - adjustedAxisLength;
            padAxis.minPixelPadding = padAxis.pixelPadding / 2;

            fixTo = padAxis.fixTo;
            if (fixTo) {
                fixDiff = fixTo[1] - padAxis.toValue(fixTo[0], true);
                fixDiff *= padAxis.transA;
                if (Math.abs(fixDiff) > padAxis.minPixelPadding || (padAxis.min === padAxis.dataMin && padAxis.max === padAxis.dataMax)) { // zooming out again, keep within restricted area
                    fixDiff = 0;
                }
                padAxis.minPixelPadding -= fixDiff;
            }
        }
    });

    /**
     * Override Axis.render in order to delete the fixTo prop
     */
    wrap(Axis.prototype, 'render', function (proceed) {
        proceed.call(this);
        this.fixTo = null;
    });


    /**
     * The ColorAxis object for inclusion in gradient legends
     */
    var ColorAxis = Highcharts.ColorAxis = function () {
        this.isColorAxis = true;
        this.init.apply(this, arguments);
    };
    extend(ColorAxis.prototype, Axis.prototype);
    extend(ColorAxis.prototype, {
        defaultColorAxisOptions: {
            lineWidth: 0,
            minPadding: 0,
            maxPadding: 0,
            gridLineWidth: 1,
            tickPixelInterval: 72,
            startOnTick: true,
            endOnTick: true,
            offset: 0,
            marker: {
                animation: {
                    duration: 50
                },
                color: 'gray',
                width: 0.01
            },
            labels: {
                overflow: 'justify'
            },
            minColor: '#EFEFFF',
            maxColor: '#003875',
            tickLength: 5
        },
        init: function (chart, userOptions) {
            var horiz = chart.options.legend.layout !== 'vertical',
                options;

            // Build the options
            options = merge(this.defaultColorAxisOptions, {
                side: horiz ? 2 : 1,
                reversed: !horiz
            }, userOptions, {
                opposite: !horiz,
                showEmpty: false,
                title: null,
                isColor: true
            });

            Axis.prototype.init.call(this, chart, options);

            // Base init() pushes it to the xAxis array, now pop it again
            //chart[this.isXAxis ? 'xAxis' : 'yAxis'].pop();

            // Prepare data classes
            if (userOptions.dataClasses) {
                this.initDataClasses(userOptions);
            }
            this.initStops(userOptions);

            // Override original axis properties
            this.horiz = horiz;
            this.zoomEnabled = false;
        },

        /*
         * Return an intermediate color between two colors, according to pos where 0
         * is the from color and 1 is the to color.
         * NOTE: Changes here should be copied
         * to the same function in drilldown.src.js and solid-gauge-src.js.
         */
        tweenColors: function (from, to, pos) {
            // Check for has alpha, because rgba colors perform worse due to lack of
            // support in WebKit.
            var hasAlpha,
                ret;

            // Unsupported color, return to-color (#3920)
            if (!to.rgba.length || !from.rgba.length) {
                ret = to.input || 'none';

            // Interpolate
            } else {
                from = from.rgba;
                to = to.rgba;
                hasAlpha = (to[3] !== 1 || from[3] !== 1);
                ret = (hasAlpha ? 'rgba(' : 'rgb(') +
                    Math.round(to[0] + (from[0] - to[0]) * (1 - pos)) + ',' +
                    Math.round(to[1] + (from[1] - to[1]) * (1 - pos)) + ',' +
                    Math.round(to[2] + (from[2] - to[2]) * (1 - pos)) +
                    (hasAlpha ? (',' + (to[3] + (from[3] - to[3]) * (1 - pos))) : '') + ')';
            }
            return ret;
        },

        initDataClasses: function (userOptions) {
            var axis = this,
                chart = this.chart,
                dataClasses,
                colorCounter = 0,
                options = this.options,
                len = userOptions.dataClasses.length;
            this.dataClasses = dataClasses = [];
            this.legendItems = [];

            each(userOptions.dataClasses, function (dataClass, i) {
                var colors;

                dataClass = merge(dataClass);
                dataClasses.push(dataClass);
                if (!dataClass.color) {
                    if (options.dataClassColor === 'category') {
                        colors = chart.options.colors;
                        dataClass.color = colors[colorCounter++];
                        // loop back to zero
                        if (colorCounter === colors.length) {
                            colorCounter = 0;
                        }
                    } else {
                        dataClass.color = axis.tweenColors(
                            Color(options.minColor),
                            Color(options.maxColor),
                            len < 2 ? 0.5 : i / (len - 1) // #3219
                        );
                    }
                }
            });
        },

        initStops: function (userOptions) {
            this.stops = userOptions.stops || [
                [0, this.options.minColor],
                [1, this.options.maxColor]
            ];
            each(this.stops, function (stop) {
                stop.color = Color(stop[1]);
            });
        },

        /**
         * Extend the setOptions method to process extreme colors and color
         * stops.
         */
        setOptions: function (userOptions) {
            Axis.prototype.setOptions.call(this, userOptions);

            this.options.crosshair = this.options.marker;
            this.coll = 'colorAxis';
        },

        setAxisSize: function () {
            var symbol = this.legendSymbol,
                chart = this.chart,
                x,
                y,
                width,
                height;

            if (symbol) {
                this.left = x = symbol.attr('x');
                this.top = y = symbol.attr('y');
                this.width = width = symbol.attr('width');
                this.height = height = symbol.attr('height');
                this.right = chart.chartWidth - x - width;
                this.bottom = chart.chartHeight - y - height;

                this.len = this.horiz ? width : height;
                this.pos = this.horiz ? x : y;
            }
        },

        /**
         * Translate from a value to a color
         */
        toColor: function (value, point) {
            var pos,
                stops = this.stops,
                from,
                to,
                color,
                dataClasses = this.dataClasses,
                dataClass,
                i;

            if (dataClasses) {
                i = dataClasses.length;
                while (i--) {
                    dataClass = dataClasses[i];
                    from = dataClass.from;
                    to = dataClass.to;
                    if ((from === UNDEFINED || value >= from) && (to === UNDEFINED || value <= to)) {
                        color = dataClass.color;
                        if (point) {
                            point.dataClass = i;
                        }
                        break;
                    }
                }

            } else {

                if (this.isLog) {
                    value = this.val2lin(value);
                }
                pos = 1 - ((this.max - value) / ((this.max - this.min) || 1));
                i = stops.length;
                while (i--) {
                    if (pos > stops[i][0]) {
                        break;
                    }
                }
                from = stops[i] || stops[i + 1];
                to = stops[i + 1] || from;

                // The position within the gradient
                pos = 1 - (to[0] - pos) / ((to[0] - from[0]) || 1);

                color = this.tweenColors(
                    from.color,
                    to.color,
                    pos
                );
            }
            return color;
        },

        /**
         * Override the getOffset method to add the whole axis groups inside the legend.
         */
        getOffset: function () {
            var group = this.legendGroup,
                sideOffset = this.chart.axisOffset[this.side];

            if (group) {

                // Hook for the getOffset method to add groups to this parent group
                this.axisParent = group;

                // Call the base
                Axis.prototype.getOffset.call(this);

                // First time only
                if (!this.added) {

                    this.added = true;

                    this.labelLeft = 0;
                    this.labelRight = this.width;
                }
                // Reset it to avoid color axis reserving space
                this.chart.axisOffset[this.side] = sideOffset;
            }
        },

        /**
         * Create the color gradient
         */
        setLegendColor: function () {
            var grad,
                horiz = this.horiz,
                options = this.options,
                reversed = this.reversed,
                one = reversed ? 1 : 0,
                zero = reversed ? 0 : 1;

            grad = horiz ? [one, 0, zero, 0] : [0, zero, 0, one]; // #3190
            this.legendColor = {
                linearGradient: { x1: grad[0], y1: grad[1], x2: grad[2], y2: grad[3] },
                stops: options.stops || [
                    [0, options.minColor],
                    [1, options.maxColor]
                ]
            };
        },

        /**
         * The color axis appears inside the legend and has its own legend symbol
         */
        drawLegendSymbol: function (legend, item) {
            var padding = legend.padding,
                legendOptions = legend.options,
                horiz = this.horiz,
                width = pick(legendOptions.symbolWidth, horiz ? 200 : 12),
                height = pick(legendOptions.symbolHeight, horiz ? 12 : 200),
                labelPadding = pick(legendOptions.labelPadding, horiz ? 16 : 30),
                itemDistance = pick(legendOptions.itemDistance, 10);

            this.setLegendColor();

            // Create the gradient
            item.legendSymbol = this.chart.renderer.rect(
                0,
                legend.baseline - 11,
                width,
                height
            ).attr({
                zIndex: 1
            }).add(item.legendGroup);

            // Set how much space this legend item takes up
            this.legendItemWidth = width + padding + (horiz ? itemDistance : labelPadding);
            this.legendItemHeight = height + padding + (horiz ? labelPadding : 0);
        },
        /**
         * Fool the legend
         */
        setState: noop,
        visible: true,
        setVisible: noop,
        getSeriesExtremes: function () {
            var series;
            if (this.series.length) {
                series = this.series[0];
                this.dataMin = series.valueMin;
                this.dataMax = series.valueMax;
            }
        },
        drawCrosshair: function (e, point) {
            var plotX = point && point.plotX,
                plotY = point && point.plotY,
                crossPos,
                axisPos = this.pos,
                axisLen = this.len;

            if (point) {
                crossPos = this.toPixels(point[point.series.colorKey]);
                if (crossPos < axisPos) {
                    crossPos = axisPos - 2;
                } else if (crossPos > axisPos + axisLen) {
                    crossPos = axisPos + axisLen + 2;
                }

                point.plotX = crossPos;
                point.plotY = this.len - crossPos;
                Axis.prototype.drawCrosshair.call(this, e, point);
                point.plotX = plotX;
                point.plotY = plotY;

                if (this.cross) {
                    this.cross
                        .attr({
                            fill: this.crosshair.color
                        })
                        .add(this.legendGroup);
                }
            }
        },
        getPlotLinePath: function (a, b, c, d, pos) {
            return typeof pos === 'number' ? // crosshairs only // #3969 pos can be 0 !!
                (this.horiz ?
                    ['M', pos - 4, this.top - 6, 'L', pos + 4, this.top - 6, pos, this.top, 'Z'] :
                    ['M', this.left, pos, 'L', this.left - 6, pos + 6, this.left - 6, pos - 6, 'Z']
                ) :
                Axis.prototype.getPlotLinePath.call(this, a, b, c, d);
        },

        update: function (newOptions, redraw) {
            var chart = this.chart,
                legend = chart.legend;

            each(this.series, function (series) {
                series.isDirtyData = true; // Needed for Axis.update when choropleth colors change
            });

            // When updating data classes, destroy old items and make sure new ones are created (#3207)
            if (newOptions.dataClasses && legend.allItems) {
                each(legend.allItems, function (item) {
                    if (item.isDataClass) {
                        item.legendGroup.destroy();
                    }
                });
                chart.isDirtyLegend = true;
            }

            // Keep the options structure updated for export. Unlike xAxis and yAxis, the colorAxis is
            // not an array. (#3207)
            chart.options[this.coll] = merge(this.userOptions, newOptions);

            Axis.prototype.update.call(this, newOptions, redraw);
            if (this.legendItem) {
                this.setLegendColor();
                legend.colorizeItem(this, true);
            }
        },

        /**
         * Get the legend item symbols for data classes
         */
        getDataClassLegendSymbols: function () {
            var axis = this,
                chart = this.chart,
                legendItems = this.legendItems,
                legendOptions = chart.options.legend,
                valueDecimals = legendOptions.valueDecimals,
                valueSuffix = legendOptions.valueSuffix || '',
                name;

            if (!legendItems.length) {
                each(this.dataClasses, function (dataClass, i) {
                    var vis = true,
                        from = dataClass.from,
                        to = dataClass.to;

                    // Assemble the default name. This can be overridden by legend.options.labelFormatter
                    name = '';
                    if (from === UNDEFINED) {
                        name = '< ';
                    } else if (to === UNDEFINED) {
                        name = '> ';
                    }
                    if (from !== UNDEFINED) {
                        name += Highcharts.numberFormat(from, valueDecimals) + valueSuffix;
                    }
                    if (from !== UNDEFINED && to !== UNDEFINED) {
                        name += ' - ';
                    }
                    if (to !== UNDEFINED) {
                        name += Highcharts.numberFormat(to, valueDecimals) + valueSuffix;
                    }

                    // Add a mock object to the legend items
                    legendItems.push(extend({
                        chart: chart,
                        name: name,
                        options: {},
                        drawLegendSymbol: LegendSymbolMixin.drawRectangle,
                        visible: true,
                        setState: noop,
                        isDataClass: true,
                        setVisible: function () {
                            vis = this.visible = !vis;
                            each(axis.series, function (series) {
                                each(series.points, function (point) {
                                    if (point.dataClass === i) {
                                        point.setVisible(vis);
                                    }
                                });
                            });

                            chart.legend.colorizeItem(this, vis);
                        }
                    }, dataClass));
                });
            }
            return legendItems;
        },
        name: '' // Prevents 'undefined' in legend in IE8
    });

    /**
     * Handle animation of the color attributes directly
     */
    each(['fill', 'stroke'], function (prop) {
        Highcharts.addAnimSetter(prop, function (fx) {
            fx.elem.attr(prop, ColorAxis.prototype.tweenColors(Color(fx.start), Color(fx.end), fx.pos));
        });
    });

    /**
     * Extend the chart getAxes method to also get the color axis
     */
    wrap(Chart.prototype, 'getAxes', function (proceed) {

        var options = this.options,
            colorAxisOptions = options.colorAxis;

        proceed.call(this);

        this.colorAxis = [];
        if (colorAxisOptions) {
            new ColorAxis(this, colorAxisOptions); // eslint-disable-line no-new
        }
    });


    /**
     * Wrap the legend getAllItems method to add the color axis. This also removes the
     * axis' own series to prevent them from showing up individually.
     */
    wrap(Legend.prototype, 'getAllItems', function (proceed) {
        var allItems = [],
            colorAxis = this.chart.colorAxis[0];

        if (colorAxis) {

            // Data classes
            if (colorAxis.options.dataClasses) {
                allItems = allItems.concat(colorAxis.getDataClassLegendSymbols());
            // Gradient legend
            } else {
                // Add this axis on top
                allItems.push(colorAxis);
            }

            // Don't add the color axis' series
            each(colorAxis.series, function (series) {
                series.options.showInLegend = false;
            });
        }

        return allItems.concat(proceed.call(this));
    });
    /**
     * Mixin for maps and heatmaps
     */
    var colorPointMixin = {
        /**
         * Set the visibility of a single point
         */
        setVisible: function (vis) {
            var point = this,
                method = vis ? 'show' : 'hide';

            // Show and hide associated elements
            each(['graphic', 'dataLabel'], function (key) {
                if (point[key]) {
                    point[key][method]();
                }
            });
        }
    };
    var colorSeriesMixin = {

        pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
            stroke: 'borderColor',
            'stroke-width': 'borderWidth',
            fill: 'color',
            dashstyle: 'dashStyle'
        },
        pointArrayMap: ['value'],
        axisTypes: ['xAxis', 'yAxis', 'colorAxis'],
        optionalAxis: 'colorAxis',
        trackerGroups: ['group', 'markerGroup', 'dataLabelsGroup'],
        getSymbol: noop,
        parallelArrays: ['x', 'y', 'value'],
        colorKey: 'value',

        /**
         * In choropleth maps, the color is a result of the value, so this needs translation too
         */
        translateColors: function () {
            var series = this,
                nullColor = this.options.nullColor,
                colorAxis = this.colorAxis,
                colorKey = this.colorKey;

            each(this.data, function (point) {
                var value = point[colorKey],
                    color;

                color = point.options.color ||
                    (value === null ? nullColor : (colorAxis && value !== undefined) ? colorAxis.toColor(value, point) : point.color || series.color);

                if (color) {
                    point.color = color;
                }
            });
        }
    };

    // Add events to the Chart object itself
    extend(Chart.prototype, {
        renderMapNavigation: function () {
            var chart = this,
                options = this.options.mapNavigation,
                buttons = options.buttons,
                n,
                button,
                buttonOptions,
                attr,
                states,
                stopEvent = function (e) {
                    if (e) {
                        if (e.preventDefault) {
                            e.preventDefault();
                        }
                        if (e.stopPropagation) {
                            e.stopPropagation();
                        }
                        e.cancelBubble = true;
                    }
                },
                outerHandler = function (e) {
                    this.handler.call(chart, e);
                    stopEvent(e); // Stop default click event (#4444)
                };

            if (pick(options.enableButtons, options.enabled) && !chart.renderer.forExport) {
                for (n in buttons) {
                    if (buttons.hasOwnProperty(n)) {
                        buttonOptions = merge(options.buttonOptions, buttons[n]);
                        attr = buttonOptions.theme;
                        attr.style = merge(buttonOptions.theme.style, buttonOptions.style); // #3203
                        states = attr.states;
                        button = chart.renderer.button(
                                buttonOptions.text,
                                0,
                                0,
                                outerHandler,
                                attr,
                                states && states.hover,
                                states && states.select,
                                0,
                                n === 'zoomIn' ? 'topbutton' : 'bottombutton'
                            )
                            .attr({
                                width: buttonOptions.width,
                                height: buttonOptions.height,
                                title: chart.options.lang[n],
                                zIndex: 5
                            })
                            .add();
                        button.handler = buttonOptions.onclick;
                        button.align(extend(buttonOptions, { width: button.width, height: 2 * button.height }), null, buttonOptions.alignTo);
                        addEvent(button.element, 'dblclick', stopEvent); // Stop double click event (#4444)
                    }
                }
            }
        },

        /**
         * Fit an inner box to an outer. If the inner box overflows left or right, align it to the sides of the
         * outer. If it overflows both sides, fit it within the outer. This is a pattern that occurs more places
         * in Highcharts, perhaps it should be elevated to a common utility function.
         */
        fitToBox: function (inner, outer) {
            each([['x', 'width'], ['y', 'height']], function (dim) {
                var pos = dim[0],
                    size = dim[1];

                if (inner[pos] + inner[size] > outer[pos] + outer[size]) { // right overflow
                    if (inner[size] > outer[size]) { // the general size is greater, fit fully to outer
                        inner[size] = outer[size];
                        inner[pos] = outer[pos];
                    } else { // align right
                        inner[pos] = outer[pos] + outer[size] - inner[size];
                    }
                }
                if (inner[size] > outer[size]) {
                    inner[size] = outer[size];
                }
                if (inner[pos] < outer[pos]) {
                    inner[pos] = outer[pos];
                }
            });


            return inner;
        },

        /**
         * Zoom the map in or out by a certain amount. Less than 1 zooms in, greater than 1 zooms out.
         */
        mapZoom: function (howMuch, centerXArg, centerYArg, mouseX, mouseY) {
            /*if (this.isMapZooming) {
                this.mapZoomQueue = arguments;
                return;
            }*/

            var chart = this,
                xAxis = chart.xAxis[0],
                xRange = xAxis.max - xAxis.min,
                centerX = pick(centerXArg, xAxis.min + xRange / 2),
                newXRange = xRange * howMuch,
                yAxis = chart.yAxis[0],
                yRange = yAxis.max - yAxis.min,
                centerY = pick(centerYArg, yAxis.min + yRange / 2),
                newYRange = yRange * howMuch,
                fixToX = mouseX ? ((mouseX - xAxis.pos) / xAxis.len) : 0.5,
                fixToY = mouseY ? ((mouseY - yAxis.pos) / yAxis.len) : 0.5,
                newXMin = centerX - newXRange * fixToX,
                newYMin = centerY - newYRange * fixToY,
                newExt = chart.fitToBox({
                    x: newXMin,
                    y: newYMin,
                    width: newXRange,
                    height: newYRange
                }, {
                    x: xAxis.dataMin,
                    y: yAxis.dataMin,
                    width: xAxis.dataMax - xAxis.dataMin,
                    height: yAxis.dataMax - yAxis.dataMin
                });

            // When mousewheel zooming, fix the point under the mouse
            if (mouseX) {
                xAxis.fixTo = [mouseX - xAxis.pos, centerXArg];
            }
            if (mouseY) {
                yAxis.fixTo = [mouseY - yAxis.pos, centerYArg];
            }

            // Zoom
            if (howMuch !== undefined) {
                xAxis.setExtremes(newExt.x, newExt.x + newExt.width, false);
                yAxis.setExtremes(newExt.y, newExt.y + newExt.height, false);

            // Reset zoom
            } else {
                xAxis.setExtremes(undefined, undefined, false);
                yAxis.setExtremes(undefined, undefined, false);
            }

            // Prevent zooming until this one is finished animating
            /*chart.holdMapZoom = true;
            setTimeout(function () {
                chart.holdMapZoom = false;
            }, 200);*/
            /*delay = animation ? animation.duration || 500 : 0;
            if (delay) {
                chart.isMapZooming = true;
                setTimeout(function () {
                    chart.isMapZooming = false;
                    if (chart.mapZoomQueue) {
                        chart.mapZoom.apply(chart, chart.mapZoomQueue);
                    }
                    chart.mapZoomQueue = null;
                }, delay);
            }*/

            chart.redraw();
        }
    });

    /**
     * Extend the Chart.render method to add zooming and panning
     */
    wrap(Chart.prototype, 'render', function (proceed) {
        var chart = this,
            mapNavigation = chart.options.mapNavigation;

        // Render the plus and minus buttons. Doing this before the shapes makes getBBox much quicker, at least in Chrome.
        chart.renderMapNavigation();

        proceed.call(chart);

        // Add the double click event
        if (pick(mapNavigation.enableDoubleClickZoom, mapNavigation.enabled) || mapNavigation.enableDoubleClickZoomTo) {
            addEvent(chart.container, 'dblclick', function (e) {
                chart.pointer.onContainerDblClick(e);
            });
        }

        // Add the mousewheel event
        if (pick(mapNavigation.enableMouseWheelZoom, mapNavigation.enabled)) {
            addEvent(chart.container, document.onmousewheel === undefined ? 'DOMMouseScroll' : 'mousewheel', function (e) {
                chart.pointer.onContainerMouseWheel(e);
                return false;
            });
        }
    });


    // Extend the Pointer
    extend(Pointer.prototype, {

        /**
         * The event handler for the doubleclick event
         */
        onContainerDblClick: function (e) {
            var chart = this.chart;

            e = this.normalize(e);

            if (chart.options.mapNavigation.enableDoubleClickZoomTo) {
                if (chart.pointer.inClass(e.target, 'highcharts-tracker')) {
                    chart.hoverPoint.zoomTo();
                }
            } else if (chart.isInsidePlot(e.chartX - chart.plotLeft, e.chartY - chart.plotTop)) {
                chart.mapZoom(
                    0.5,
                    chart.xAxis[0].toValue(e.chartX),
                    chart.yAxis[0].toValue(e.chartY),
                    e.chartX,
                    e.chartY
                );
            }
        },

        /**
         * The event handler for the mouse scroll event
         */
        onContainerMouseWheel: function (e) {
            var chart = this.chart,
                delta;

            e = this.normalize(e);

            // Firefox uses e.detail, WebKit and IE uses wheelDelta
            delta = e.detail || -(e.wheelDelta / 120);
            if (chart.isInsidePlot(e.chartX - chart.plotLeft, e.chartY - chart.plotTop)) {
                chart.mapZoom(
                    //delta > 0 ? 2 : 0.5,
                    Math.pow(2, delta),
                    chart.xAxis[0].toValue(e.chartX),
                    chart.yAxis[0].toValue(e.chartY),
                    e.chartX,
                    e.chartY
                );
            }
        }
    });

    // Implement the pinchType option
    wrap(Pointer.prototype, 'init', function (proceed, chart, options) {

        proceed.call(this, chart, options);

        // Pinch status
        if (pick(options.mapNavigation.enableTouchZoom, options.mapNavigation.enabled)) {
            this.pinchX = this.pinchHor = this.pinchY = this.pinchVert = this.hasZoom = true;
        }
    });

    // Extend the pinchTranslate method to preserve fixed ratio when zooming
    wrap(Pointer.prototype, 'pinchTranslate', function (proceed, pinchDown, touches, transform, selectionMarker, clip, lastValidTouch) {
        var xBigger;
        proceed.call(this, pinchDown, touches, transform, selectionMarker, clip, lastValidTouch);

        // Keep ratio
        if (this.chart.options.chart.type === 'map' && this.hasZoom) {
            xBigger = transform.scaleX > transform.scaleY;
            this.pinchTranslateDirection(
                !xBigger,
                pinchDown,
                touches,
                transform,
                selectionMarker,
                clip,
                lastValidTouch,
                xBigger ? transform.scaleX : transform.scaleY
            );
        }
    });


    // The vector-effect attribute is not supported in IE <= 11 (at least), so we need
    // diffent logic (#3218)
    var supportsVectorEffect = document.documentElement.style.vectorEffect !== undefined;

    /**
     * Extend the default options with map options
     */
    defaultPlotOptions.map = merge(defaultPlotOptions.scatter, {
        allAreas: true,

        animation: false, // makes the complex shapes slow
        nullColor: '#F8F8F8',
        borderColor: 'silver',
        borderWidth: 1,
        marker: null,
        stickyTracking: false,
        dataLabels: {
            formatter: function () { // #2945
                return this.point.value;
            },
            inside: true, // for the color
            verticalAlign: 'middle',
            crop: false,
            overflow: false,
            padding: 0
        },
        turboThreshold: 0,
        tooltip: {
            followPointer: true,
            pointFormat: '{point.name}: {point.value}<br/>'
        },
        states: {
            normal: {
                animation: true
            },
            hover: {
                brightness: 0.2,
                halo: null
            }
        }
    });

    /**
     * The MapAreaPoint object
     */
    var MapAreaPoint = extendClass(Point, extend({
        /**
         * Extend the Point object to split paths
         */
        applyOptions: function (options, x) {

            var point = Point.prototype.applyOptions.call(this, options, x),
                series = this.series,
                joinBy = series.joinBy,
                mapPoint;

            if (series.mapData) {
                mapPoint = point[joinBy[1]] !== undefined && series.mapMap[point[joinBy[1]]];
                if (mapPoint) {
                    // This applies only to bubbles
                    if (series.xyFromShape) {
                        point.x = mapPoint._midX;
                        point.y = mapPoint._midY;
                    }
                    extend(point, mapPoint); // copy over properties
                } else {
                    point.value = point.value || null;
                }
            }

            return point;
        },

        /**
         * Stop the fade-out
         */
        onMouseOver: function (e) {
            clearTimeout(this.colorInterval);
            if (this.value !== null) {
                Point.prototype.onMouseOver.call(this, e);
            } else { //#3401 Tooltip doesn't hide when hovering over null points
                this.series.onMouseOut(e);
            }
        },
        /**
         * Custom animation for tweening out the colors. Animation reduces blinking when hovering
         * over islands and coast lines. We run a custom implementation of animation becuase we
         * need to be able to run this independently from other animations like zoom redraw. Also,
         * adding color animation to the adapters would introduce almost the same amount of code.
         */
        onMouseOut: function () {
            var point = this,
                start = +new Date(),
                normalColor = Color(point.color),
                hoverColor = Color(point.pointAttr.hover.fill),
                animation = point.series.options.states.normal.animation,
                duration = animation && (animation.duration || 500),
                fill;

            if (duration && normalColor.rgba.length === 4 && hoverColor.rgba.length === 4 && point.state !== 'select') {
                fill = point.pointAttr[''].fill;
                delete point.pointAttr[''].fill; // avoid resetting it in Point.setState

                clearTimeout(point.colorInterval);
                point.colorInterval = setInterval(function () {
                    var pos = (new Date() - start) / duration,
                        graphic = point.graphic;
                    if (pos > 1) {
                        pos = 1;
                    }
                    if (graphic) {
                        graphic.attr('fill', ColorAxis.prototype.tweenColors.call(0, hoverColor, normalColor, pos));
                    }
                    if (pos >= 1) {
                        clearTimeout(point.colorInterval);
                    }
                }, 13);
            }
            Point.prototype.onMouseOut.call(point);

            if (fill) {
                point.pointAttr[''].fill = fill;
            }
        },

        /**
         * Zoom the chart to view a specific area point
         */
        zoomTo: function () {
            var point = this,
                series = point.series;

            series.xAxis.setExtremes(
                point._minX,
                point._maxX,
                false
            );
            series.yAxis.setExtremes(
                point._minY,
                point._maxY,
                false
            );
            series.chart.redraw();
        }
    }, colorPointMixin)
    );

    /**
     * Add the series type
     */
    seriesTypes.map = extendClass(seriesTypes.scatter, merge(colorSeriesMixin, {
        type: 'map',
        pointClass: MapAreaPoint,
        supportsDrilldown: true,
        getExtremesFromAll: true,
        useMapGeometry: true, // get axis extremes from paths, not values
        forceDL: true,
        searchPoint: noop,
        directTouch: true, // When tooltip is not shared, this series (and derivatives) requires direct touch/hover. KD-tree does not apply.
        preserveAspectRatio: true, // X axis and Y axis must have same translation slope
        /**
         * Get the bounding box of all paths in the map combined.
         */
        getBox: function (paths) {
            var MAX_VALUE = Number.MAX_VALUE,
                maxX = -MAX_VALUE,
                minX =  MAX_VALUE,
                maxY = -MAX_VALUE,
                minY =  MAX_VALUE,
                minRange = MAX_VALUE,
                xAxis = this.xAxis,
                yAxis = this.yAxis,
                hasBox;

            // Find the bounding box
            each(paths || [], function (point) {

                if (point.path) {
                    if (typeof point.path === 'string') {
                        point.path = Highcharts.splitPath(point.path);
                    }

                    var path = point.path || [],
                        i = path.length,
                        even = false, // while loop reads from the end
                        pointMaxX = -MAX_VALUE,
                        pointMinX =  MAX_VALUE,
                        pointMaxY = -MAX_VALUE,
                        pointMinY =  MAX_VALUE,
                        properties = point.properties;

                    // The first time a map point is used, analyze its box
                    if (!point._foundBox) {
                        while (i--) {
                            if (typeof path[i] === 'number' && !isNaN(path[i])) {
                                if (even) { // even = x
                                    pointMaxX = Math.max(pointMaxX, path[i]);
                                    pointMinX = Math.min(pointMinX, path[i]);
                                } else { // odd = Y
                                    pointMaxY = Math.max(pointMaxY, path[i]);
                                    pointMinY = Math.min(pointMinY, path[i]);
                                }
                                even = !even;
                            }
                        }
                        // Cache point bounding box for use to position data labels, bubbles etc
                        point._midX = pointMinX + (pointMaxX - pointMinX) *
                            (point.middleX || (properties && properties['hc-middle-x']) || 0.5); // pick is slower and very marginally needed
                        point._midY = pointMinY + (pointMaxY - pointMinY) *
                            (point.middleY || (properties && properties['hc-middle-y']) || 0.5);
                        point._maxX = pointMaxX;
                        point._minX = pointMinX;
                        point._maxY = pointMaxY;
                        point._minY = pointMinY;
                        point.labelrank = pick(point.labelrank, (pointMaxX - pointMinX) * (pointMaxY - pointMinY));
                        point._foundBox = true;
                    }

                    maxX = Math.max(maxX, point._maxX);
                    minX = Math.min(minX, point._minX);
                    maxY = Math.max(maxY, point._maxY);
                    minY = Math.min(minY, point._minY);
                    minRange = Math.min(point._maxX - point._minX, point._maxY - point._minY, minRange);
                    hasBox = true;
                }
            });

            // Set the box for the whole series
            if (hasBox) {
                this.minY = Math.min(minY, pick(this.minY, MAX_VALUE));
                this.maxY = Math.max(maxY, pick(this.maxY, -MAX_VALUE));
                this.minX = Math.min(minX, pick(this.minX, MAX_VALUE));
                this.maxX = Math.max(maxX, pick(this.maxX, -MAX_VALUE));

                // If no minRange option is set, set the default minimum zooming range to 5 times the
                // size of the smallest element
                if (xAxis && xAxis.options.minRange === undefined) {
                    xAxis.minRange = Math.min(5 * minRange, (this.maxX - this.minX) / 5, xAxis.minRange || MAX_VALUE);
                }
                if (yAxis && yAxis.options.minRange === undefined) {
                    yAxis.minRange = Math.min(5 * minRange, (this.maxY - this.minY) / 5, yAxis.minRange || MAX_VALUE);
                }
            }
        },

        getExtremes: function () {
            // Get the actual value extremes for colors
            Series.prototype.getExtremes.call(this, this.valueData);

            // Recalculate box on updated data
            if (this.chart.hasRendered && this.isDirtyData) {
                this.getBox(this.options.data);
            }

            this.valueMin = this.dataMin;
            this.valueMax = this.dataMax;

            // Extremes for the mock Y axis
            this.dataMin = this.minY;
            this.dataMax = this.maxY;
        },

        /**
         * Translate the path so that it automatically fits into the plot area box
         * @param {Object} path
         */
        translatePath: function (path) {

            var series = this,
                even = false, // while loop reads from the end
                xAxis = series.xAxis,
                yAxis = series.yAxis,
                xMin = xAxis.min,
                xTransA = xAxis.transA,
                xMinPixelPadding = xAxis.minPixelPadding,
                yMin = yAxis.min,
                yTransA = yAxis.transA,
                yMinPixelPadding = yAxis.minPixelPadding,
                i,
                ret = []; // Preserve the original

            // Do the translation
            if (path) {
                i = path.length;
                while (i--) {
                    if (typeof path[i] === 'number') {
                        ret[i] = even ?
                            (path[i] - xMin) * xTransA + xMinPixelPadding :
                            (path[i] - yMin) * yTransA + yMinPixelPadding;
                        even = !even;
                    } else {
                        ret[i] = path[i];
                    }
                }
            }

            return ret;
        },

        /**
         * Extend setData to join in mapData. If the allAreas option is true, all areas
         * from the mapData are used, and those that don't correspond to a data value
         * are given null values.
         */
        setData: function (data, redraw) {
            var options = this.options,
                mapData = options.mapData,
                joinBy = options.joinBy,
                joinByNull = joinBy === null,
                dataUsed = [],
                mapPoint,
                transform,
                mapTransforms,
                props,
                i;

            if (joinByNull) {
                joinBy = '_i';
            }
            joinBy = this.joinBy = Highcharts.splat(joinBy);
            if (!joinBy[1]) {
                joinBy[1] = joinBy[0];
            }

            // Pick up numeric values, add index
            if (data) {
                each(data, function (val, i) {
                    if (typeof val === 'number') {
                        data[i] = {
                            value: val
                        };
                    }
                    if (joinByNull) {
                        data[i]._i = i;
                    }
                });
            }

            this.getBox(data);
            if (mapData) {
                if (mapData.type === 'FeatureCollection') {
                    if (mapData['hc-transform']) {
                        this.chart.mapTransforms = mapTransforms = mapData['hc-transform'];
                        // Cache cos/sin of transform rotation angle
                        for (transform in mapTransforms) {
                            if (mapTransforms.hasOwnProperty(transform) && transform.rotation) {
                                transform.cosAngle = Math.cos(transform.rotation);
                                transform.sinAngle = Math.sin(transform.rotation);
                            }
                        }
                    }
                    mapData = Highcharts.geojson(mapData, this.type, this);
                }

                this.getBox(mapData);
                this.mapData = mapData;
                this.mapMap = {};

                for (i = 0; i < mapData.length; i++) {
                    mapPoint = mapData[i];
                    props = mapPoint.properties;

                    mapPoint._i = i;
                    // Copy the property over to root for faster access
                    if (joinBy[0] && props && props[joinBy[0]]) {
                        mapPoint[joinBy[0]] = props[joinBy[0]];
                    }
                    this.mapMap[mapPoint[joinBy[0]]] = mapPoint;
                }

                if (options.allAreas) {

                    data = data || [];

                    // Registered the point codes that actually hold data
                    if (joinBy[1]) {
                        each(data, function (point) {
                            dataUsed.push(point[joinBy[1]]);
                        });
                    }

                    // Add those map points that don't correspond to data, which will be drawn as null points
                    dataUsed = '|' + dataUsed.join('|') + '|'; // String search is faster than array.indexOf

                    each(mapData, function (mapPoint) {
                        if (!joinBy[0] || dataUsed.indexOf('|' + mapPoint[joinBy[0]] + '|') === -1) {
                            data.push(merge(mapPoint, { value: null }));
                        }
                    });
                }
            }
            Series.prototype.setData.call(this, data, redraw);
        },


        /**
         * No graph for the map series
         */
        drawGraph: noop,

        /**
         * We need the points' bounding boxes in order to draw the data labels, so
         * we skip it now and call it from drawPoints instead.
         */
        drawDataLabels: noop,

        /**
         * Allow a quick redraw by just translating the area group. Used for zooming and panning
         * in capable browsers.
         */
        doFullTranslate: function () {
            return this.isDirtyData || this.chart.isResizing || this.chart.renderer.isVML || !this.baseTrans;
        },

        /**
         * Add the path option for data points. Find the max value for color calculation.
         */
        translate: function () {
            var series = this,
                xAxis = series.xAxis,
                yAxis = series.yAxis,
                doFullTranslate = series.doFullTranslate();

            series.generatePoints();

            each(series.data, function (point) {

                // Record the middle point (loosely based on centroid), determined
                // by the middleX and middleY options.
                point.plotX = xAxis.toPixels(point._midX, true);
                point.plotY = yAxis.toPixels(point._midY, true);

                if (doFullTranslate) {

                    point.shapeType = 'path';
                    point.shapeArgs = {
                        d: series.translatePath(point.path)
                    };
                    if (supportsVectorEffect) {
                        point.shapeArgs['vector-effect'] = 'non-scaling-stroke';
                    }
                }
            });

            series.translateColors();
        },

        /**
         * Use the drawPoints method of column, that is able to handle simple shapeArgs.
         * Extend it by assigning the tooltip position.
         */
        drawPoints: function () {
            var series = this,
                xAxis = series.xAxis,
                yAxis = series.yAxis,
                group = series.group,
                chart = series.chart,
                renderer = chart.renderer,
                scaleX,
                scaleY,
                translateX,
                translateY,
                baseTrans = this.baseTrans;

            // Set a group that handles transform during zooming and panning in order to preserve clipping
            // on series.group
            if (!series.transformGroup) {
                series.transformGroup = renderer.g()
                    .attr({
                        scaleX: 1,
                        scaleY: 1
                    })
                    .add(group);
                series.transformGroup.survive = true;
            }

            // Draw the shapes again
            if (series.doFullTranslate()) {

                // Individual point actions
                if (chart.hasRendered && series.pointAttrToOptions.fill === 'color') {
                    each(series.points, function (point) {

                        // Reset color on update/redraw
                        if (point.shapeArgs) {
                            point.shapeArgs.fill = point.pointAttr[pick(point.state, '')].fill; // #3529
                        }
                    });
                }

                // If vector-effect is not supported, we set the stroke-width on the group element
                // and let all point graphics inherit. That way we don't have to iterate over all
                // points to update the stroke-width on zooming.
                if (!supportsVectorEffect) {
                    each(series.points, function (point) {
                        var attr = point.pointAttr[''];
                        if (attr['stroke-width'] === series.pointAttr['']['stroke-width']) {
                            attr['stroke-width'] = 'inherit';
                        }
                    });
                }

                // Draw them in transformGroup
                series.group = series.transformGroup;
                seriesTypes.column.prototype.drawPoints.apply(series);
                series.group = group; // Reset

                // Add class names
                each(series.points, function (point) {
                    if (point.graphic) {
                        if (point.name) {
                            point.graphic.addClass('highcharts-name-' + point.name.replace(' ', '-').toLowerCase());
                        }
                        if (point.properties && point.properties['hc-key']) {
                            point.graphic.addClass('highcharts-key-' + point.properties['hc-key'].toLowerCase());
                        }

                        if (!supportsVectorEffect) {
                            point.graphic['stroke-widthSetter'] = noop;
                        }
                    }
                });

                // Set the base for later scale-zooming. The originX and originY properties are the
                // axis values in the plot area's upper left corner.
                this.baseTrans = {
                    originX: xAxis.min - xAxis.minPixelPadding / xAxis.transA,
                    originY: yAxis.min - yAxis.minPixelPadding / yAxis.transA + (yAxis.reversed ? 0 : yAxis.len / yAxis.transA),
                    transAX: xAxis.transA,
                    transAY: yAxis.transA
                };

                // Reset transformation in case we're doing a full translate (#3789)
                this.transformGroup.animate({
                    translateX: 0,
                    translateY: 0,
                    scaleX: 1,
                    scaleY: 1
                });

            // Just update the scale and transform for better performance
            } else {
                scaleX = xAxis.transA / baseTrans.transAX;
                scaleY = yAxis.transA / baseTrans.transAY;
                translateX = xAxis.toPixels(baseTrans.originX, true);
                translateY = yAxis.toPixels(baseTrans.originY, true);

                // Handle rounding errors in normal view (#3789)
                if (scaleX > 0.99 && scaleX < 1.01 && scaleY > 0.99 && scaleY < 1.01) {
                    scaleX = 1;
                    scaleY = 1;
                    translateX = Math.round(translateX);
                    translateY = Math.round(translateY);
                }

                this.transformGroup.animate({
                    translateX: translateX,
                    translateY: translateY,
                    scaleX: scaleX,
                    scaleY: scaleY
                });

            }

            // Set the stroke-width directly on the group element so the children inherit it. We need to use
            // setAttribute directly, because the stroke-widthSetter method expects a stroke color also to be
            // set.
            if (!supportsVectorEffect) {
                series.group.element.setAttribute('stroke-width', series.options.borderWidth / (scaleX || 1));
            }

            this.drawMapDataLabels();


        },

        /**
         * Draw the data labels. Special for maps is the time that the data labels are drawn (after points),
         * and the clipping of the dataLabelsGroup.
         */
        drawMapDataLabels: function () {

            Series.prototype.drawDataLabels.call(this);
            if (this.dataLabelsGroup) {
                this.dataLabelsGroup.clip(this.chart.clipRect);
            }
        },

        /**
         * Override render to throw in an async call in IE8. Otherwise it chokes on the US counties demo.
         */
        render: function () {
            var series = this,
                render = Series.prototype.render;

            // Give IE8 some time to breathe.
            if (series.chart.renderer.isVML && series.data.length > 3000) {
                setTimeout(function () {
                    render.call(series);
                });
            } else {
                render.call(series);
            }
        },

        /**
         * The initial animation for the map series. By default, animation is disabled.
         * Animation of map shapes is not at all supported in VML browsers.
         */
        animate: function (init) {
            var chart = this.chart,
                animation = this.options.animation,
                group = this.group,
                xAxis = this.xAxis,
                yAxis = this.yAxis,
                left = xAxis.pos,
                top = yAxis.pos;

            if (chart.renderer.isSVG) {

                if (animation === true) {
                    animation = {
                        duration: 1000
                    };
                }

                // Initialize the animation
                if (init) {

                    // Scale down the group and place it in the center
                    group.attr({
                        translateX: left + xAxis.len / 2,
                        translateY: top + yAxis.len / 2,
                        scaleX: 0.001, // #1499
                        scaleY: 0.001
                    });

                // Run the animation
                } else {
                    group.animate({
                        translateX: left,
                        translateY: top,
                        scaleX: 1,
                        scaleY: 1
                    }, animation);

                    // Delete this function to allow it only once
                    this.animate = null;
                }
            }
        },

        /**
         * Animate in the new series from the clicked point in the old series.
         * Depends on the drilldown.js module
         */
        animateDrilldown: function (init) {
            var toBox = this.chart.plotBox,
                level = this.chart.drilldownLevels[this.chart.drilldownLevels.length - 1],
                fromBox = level.bBox,
                animationOptions = this.chart.options.drilldown.animation,
                scale;

            if (!init) {

                scale = Math.min(fromBox.width / toBox.width, fromBox.height / toBox.height);
                level.shapeArgs = {
                    scaleX: scale,
                    scaleY: scale,
                    translateX: fromBox.x,
                    translateY: fromBox.y
                };

                each(this.points, function (point) {
                    if (point.graphic) {
                        point.graphic
                            .attr(level.shapeArgs)
                            .animate({
                                scaleX: 1,
                                scaleY: 1,
                                translateX: 0,
                                translateY: 0
                            }, animationOptions);
                    }
                });

                this.animate = null;
            }

        },

        drawLegendSymbol: LegendSymbolMixin.drawRectangle,

        /**
         * When drilling up, pull out the individual point graphics from the lower series
         * and animate them into the origin point in the upper series.
         */
        animateDrillupFrom: function (level) {
            seriesTypes.column.prototype.animateDrillupFrom.call(this, level);
        },


        /**
         * When drilling up, keep the upper series invisible until the lower series has
         * moved into place
         */
        animateDrillupTo: function (init) {
            seriesTypes.column.prototype.animateDrillupTo.call(this, init);
        }
    }));



    // The mapline series type
    defaultPlotOptions.mapline = merge(defaultPlotOptions.map, {
        lineWidth: 1,
        fillColor: 'none'
    });
    seriesTypes.mapline = extendClass(seriesTypes.map, {
        type: 'mapline',
        pointAttrToOptions: { // mapping between SVG attributes and the corresponding options
            stroke: 'color',
            'stroke-width': 'lineWidth',
            fill: 'fillColor',
            dashstyle: 'dashStyle'
        },
        drawLegendSymbol: seriesTypes.line.prototype.drawLegendSymbol
    });


    // The mappoint series type
    defaultPlotOptions.mappoint = merge(defaultPlotOptions.scatter, {
        dataLabels: {
            enabled: true,
            formatter: function () { // #2945
                return this.point.name;
            },
            crop: false,
            defer: false,
            overflow: false,
            style: {
                color: '#000000'
            }
        }
    });
    seriesTypes.mappoint = extendClass(seriesTypes.scatter, {
        type: 'mappoint',
        forceDL: true,
        pointClass: extendClass(Point, {
            applyOptions: function (options, x) {
                var point = Point.prototype.applyOptions.call(this, options, x);
                if (options.lat !== undefined && options.lon !== undefined) {
                    point = extend(point, this.series.chart.fromLatLonToPoint(point));
                }
                return point;
            }
        })
    });


    // The mapbubble series type
    if (seriesTypes.bubble) {

        defaultPlotOptions.mapbubble = merge(defaultPlotOptions.bubble, {
            animationLimit: 500,
            tooltip: {
                pointFormat: '{point.name}: {point.z}'
            }
        });
        seriesTypes.mapbubble = extendClass(seriesTypes.bubble, {
            pointClass: extendClass(Point, {
                applyOptions: function (options, x) {
                    var point;
                    if (options && options.lat !== undefined && options.lon !== undefined) {
                        point = Point.prototype.applyOptions.call(this, options, x);
                        point = extend(point, this.series.chart.fromLatLonToPoint(point));
                    } else {
                        point = MapAreaPoint.prototype.applyOptions.call(this, options, x);
                    }
                    return point;
                },
                ttBelow: false
            }),
            xyFromShape: true,
            type: 'mapbubble',
            pointArrayMap: ['z'], // If one single value is passed, it is interpreted as z
            /**
             * Return the map area identified by the dataJoinBy option
             */
            getMapData: seriesTypes.map.prototype.getMapData,
            getBox: seriesTypes.map.prototype.getBox,
            setData: seriesTypes.map.prototype.setData
        });
    }

    /**
     * Extend the default options with map options
     */
    defaultOptions.plotOptions.heatmap = merge(defaultOptions.plotOptions.scatter, {
        animation: false,
        borderWidth: 0,
        nullColor: '#F8F8F8',
        dataLabels: {
            formatter: function () { // #2945
                return this.point.value;
            },
            inside: true,
            verticalAlign: 'middle',
            crop: false,
            overflow: false,
            padding: 0 // #3837
        },
        marker: null,
        pointRange: null, // dynamically set to colsize by default
        tooltip: {
            pointFormat: '{point.x}, {point.y}: {point.value}<br/>'
        },
        states: {
            normal: {
                animation: true
            },
            hover: {
                halo: false,  // #3406, halo is not required on heatmaps
                brightness: 0.2
            }
        }
    });

    // The Heatmap series type
    seriesTypes.heatmap = extendClass(seriesTypes.scatter, merge(colorSeriesMixin, {
        type: 'heatmap',
        pointArrayMap: ['y', 'value'],
        hasPointSpecificOptions: true,
        pointClass: extendClass(Point, colorPointMixin),
        supportsDrilldown: true,
        getExtremesFromAll: true,
        directTouch: true,

        /**
         * Override the init method to add point ranges on both axes.
         */
        init: function () {
            var options;
            seriesTypes.scatter.prototype.init.apply(this, arguments);

            options = this.options;
            options.pointRange = pick(options.pointRange, options.colsize || 1); // #3758, prevent resetting in setData
            this.yAxis.axisPointRange = options.rowsize || 1; // general point range
        },
        translate: function () {
            var series = this,
                options = series.options,
                xAxis = series.xAxis,
                yAxis = series.yAxis,
                between = function (x, a, b) {
                    return Math.min(Math.max(a, x), b);
                };

            series.generatePoints();

            each(series.points, function (point) {
                var xPad = (options.colsize || 1) / 2,
                    yPad = (options.rowsize || 1) / 2,
                    x1 = between(Math.round(xAxis.len - xAxis.translate(point.x - xPad, 0, 1, 0, 1)), 0, xAxis.len),
                    x2 = between(Math.round(xAxis.len - xAxis.translate(point.x + xPad, 0, 1, 0, 1)), 0, xAxis.len),
                    y1 = between(Math.round(yAxis.translate(point.y - yPad, 0, 1, 0, 1)), 0, yAxis.len),
                    y2 = between(Math.round(yAxis.translate(point.y + yPad, 0, 1, 0, 1)), 0, yAxis.len);

                // Set plotX and plotY for use in K-D-Tree and more
                point.plotX = point.clientX = (x1 + x2) / 2;
                point.plotY = (y1 + y2) / 2;

                point.shapeType = 'rect';
                point.shapeArgs = {
                    x: Math.min(x1, x2),
                    y: Math.min(y1, y2),
                    width: Math.abs(x2 - x1),
                    height: Math.abs(y2 - y1)
                };
            });

            series.translateColors();

            // Make sure colors are updated on colorAxis update (#2893)
            if (this.chart.hasRendered) {
                each(series.points, function (point) {
                    point.shapeArgs.fill = point.options.color || point.color; // #3311
                });
            }
        },
        drawPoints: seriesTypes.column.prototype.drawPoints,
        animate: noop,
        getBox: noop,
        drawLegendSymbol: LegendSymbolMixin.drawRectangle,

        getExtremes: function () {
            // Get the extremes from the value data
            Series.prototype.getExtremes.call(this, this.valueData);
            this.valueMin = this.dataMin;
            this.valueMax = this.dataMax;

            // Get the extremes from the y data
            Series.prototype.getExtremes.call(this);
        }

    }));


    /**
     * Test for point in polygon. Polygon defined as array of [x,y] points.
     */
    function pointInPolygon(point, polygon) {
        var i, j, rel1, rel2, c = false,
            x = point.x,
            y = point.y;

        for (i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
            rel1 = polygon[i][1] > y;
            rel2 = polygon[j][1] > y;
            if (rel1 !== rel2 && (x < (polygon[j][0] - polygon[i][0]) * (y - polygon[i][1]) / (polygon[j][1] - polygon[i][1]) + polygon[i][0])) {
                c = !c;
            }
        }

        return c;
    }

    /**
     * Get point from latLon using specified transform definition
     */
    Chart.prototype.transformFromLatLon = function (latLon, transform) {
        if (window.proj4 === undefined) {
            error(21);
            return {
                x: 0,
                y: null
            };
        }

        var projected = window.proj4(transform.crs, [latLon.lon, latLon.lat]),
            cosAngle = transform.cosAngle || (transform.rotation && Math.cos(transform.rotation)),
            sinAngle = transform.sinAngle || (transform.rotation && Math.sin(transform.rotation)),
            rotated = transform.rotation ? [projected[0] * cosAngle + projected[1] * sinAngle, -projected[0] * sinAngle + projected[1] * cosAngle] : projected;

        return {
            x: ((rotated[0] - (transform.xoffset || 0)) * (transform.scale || 1) + (transform.xpan || 0)) * (transform.jsonres || 1) + (transform.jsonmarginX || 0),
            y: (((transform.yoffset || 0) - rotated[1]) * (transform.scale || 1) + (transform.ypan || 0)) * (transform.jsonres || 1) - (transform.jsonmarginY || 0)
        };
    };

    /**
     * Get latLon from point using specified transform definition
     */
    Chart.prototype.transformToLatLon = function (point, transform) {
        if (window.proj4 === undefined) {
            error(21);
            return;
        }

        var normalized = {
                x: ((point.x - (transform.jsonmarginX || 0)) / (transform.jsonres || 1) - (transform.xpan || 0)) / (transform.scale || 1) + (transform.xoffset || 0),
                y: ((-point.y - (transform.jsonmarginY || 0)) / (transform.jsonres || 1) + (transform.ypan || 0)) / (transform.scale || 1) + (transform.yoffset || 0)
            },
            cosAngle = transform.cosAngle || (transform.rotation && Math.cos(transform.rotation)),
            sinAngle = transform.sinAngle || (transform.rotation && Math.sin(transform.rotation)),
            // Note: Inverted sinAngle to reverse rotation direction
            projected = window.proj4(transform.crs, 'WGS84', transform.rotation ? {
                x: normalized.x * cosAngle + normalized.y * -sinAngle,
                y: normalized.x * sinAngle + normalized.y * cosAngle
            } : normalized);

        return { lat: projected.y, lon: projected.x };
    };

    Chart.prototype.fromPointToLatLon = function (point) {
        var transforms = this.mapTransforms,
            transform;

        if (!transforms) {
            error(22);
            return;
        }

        for (transform in transforms) {
            if (transforms.hasOwnProperty(transform) && transforms[transform].hitZone && 
                    pointInPolygon({ x: point.x, y: -point.y }, transforms[transform].hitZone.coordinates[0])) {
                return this.transformToLatLon(point, transforms[transform]);
            }
        }

        return this.transformToLatLon(point, transforms['default']); // eslint-disable-line dot-notation
    };

    Chart.prototype.fromLatLonToPoint = function (latLon) {
        var transforms = this.mapTransforms,
            transform,
            coords;

        if (!transforms) {
            error(22);
            return {
                x: 0,
                y: null
            };
        }

        for (transform in transforms) {
            if (transforms.hasOwnProperty(transform) && transforms[transform].hitZone) {
                coords = this.transformFromLatLon(latLon, transforms[transform]);
                if (pointInPolygon({ x: coords.x, y: -coords.y }, transforms[transform].hitZone.coordinates[0])) {
                    return coords;
                }
            }
        }

        return this.transformFromLatLon(latLon, transforms['default']); // eslint-disable-line dot-notation
    };

    /**
     * Convert a geojson object to map data of a given Highcharts type (map, mappoint or mapline).
     */
    Highcharts.geojson = function (geojson, hType, series) {
        var mapData = [],
            path = [],
            polygonToPath = function (polygon) {
                var i,
                    len = polygon.length;
                path.push('M');
                for (i = 0; i < len; i++) {
                    if (i === 1) {
                        path.push('L');
                    }
                    path.push(polygon[i][0], -polygon[i][1]);
                }
            };

        hType = hType || 'map';

        each(geojson.features, function (feature) {

            var geometry = feature.geometry,
                type = geometry.type,
                coordinates = geometry.coordinates,
                properties = feature.properties,
                point;

            path = [];

            if (hType === 'map' || hType === 'mapbubble') {
                if (type === 'Polygon') {
                    each(coordinates, polygonToPath);
                    path.push('Z');

                } else if (type === 'MultiPolygon') {
                    each(coordinates, function (items) {
                        each(items, polygonToPath);
                    });
                    path.push('Z');
                }

                if (path.length) {
                    point = { path: path };
                }

            } else if (hType === 'mapline') {
                if (type === 'LineString') {
                    polygonToPath(coordinates);
                } else if (type === 'MultiLineString') {
                    each(coordinates, polygonToPath);
                }

                if (path.length) {
                    point = { path: path };
                }

            } else if (hType === 'mappoint') {
                if (type === 'Point') {
                    point = {
                        x: coordinates[0],
                        y: -coordinates[1]
                    };
                }
            }
            if (point) {
                mapData.push(extend(point, {
                    name: properties.name || properties.NAME,
                    properties: properties
                }));
            }

        });

        // Create a credits text that includes map source, to be picked up in Chart.showCredits
        if (series && geojson.copyrightShort) {
            series.chart.mapCredits = '<a href="http://www.highcharts.com">Highcharts</a> \u00A9 ' +
                '<a href="' + geojson.copyrightUrl + '">' + geojson.copyrightShort + '</a>';
            series.chart.mapCreditsFull = geojson.copyright;
        }

        return mapData;
    };

    /**
     * Override showCredits to include map source by default
     */
    wrap(Chart.prototype, 'showCredits', function (proceed, credits) {

        if (defaultOptions.credits.text === this.options.credits.text && this.mapCredits) { // default text and mapCredits is set
            credits.text = this.mapCredits;
            credits.href = null;
        }

        proceed.call(this, credits);

        if (this.credits) {
            this.credits.attr({
                title: this.mapCreditsFull
            });
        }
    });


    // Add language
    extend(defaultOptions.lang, {
        zoomIn: 'Zoom in',
        zoomOut: 'Zoom out'
    });


    // Set the default map navigation options
    defaultOptions.mapNavigation = {
        buttonOptions: {
            alignTo: 'plotBox',
            align: 'left',
            verticalAlign: 'top',
            x: 0,
            width: 18,
            height: 18,
            style: {
                fontSize: '15px',
                fontWeight: 'bold',
                textAlign: 'center'
            },
            theme: {
                'stroke-width': 1
            }
        },
        buttons: {
            zoomIn: {
                onclick: function () {
                    this.mapZoom(0.5);
                },
                text: '+',
                y: 0
            },
            zoomOut: {
                onclick: function () {
                    this.mapZoom(2);
                },
                text: '-',
                y: 28
            }
        }
        // enabled: false,
        // enableButtons: null, // inherit from enabled
        // enableTouchZoom: null, // inherit from enabled
        // enableDoubleClickZoom: null, // inherit from enabled
        // enableDoubleClickZoomTo: false
        // enableMouseWheelZoom: null, // inherit from enabled
    };

    /**
     * Utility for reading SVG paths directly.
     */
    Highcharts.splitPath = function (path) {
        var i;

        // Move letters apart
        path = path.replace(/([A-Za-z])/g, ' $1 ');
        // Trim
        path = path.replace(/^\s*/, '').replace(/\s*$/, '');

        // Split on spaces and commas
        path = path.split(/[ ,]+/);

        // Parse numbers
        for (i = 0; i < path.length; i++) {
            if (!/[a-zA-Z]/.test(path[i])) {
                path[i] = parseFloat(path[i]);
            }
        }
        return path;
    };

    // A placeholder for map definitions
    Highcharts.maps = {};





    // Create symbols for the zoom buttons
    function selectiveRoundedRect(x, y, w, h, rTopLeft, rTopRight, rBottomRight, rBottomLeft) {
        return ['M', x + rTopLeft, y,
            // top side
            'L', x + w - rTopRight, y,
            // top right corner
            'C', x + w - rTopRight / 2, y, x + w, y + rTopRight / 2, x + w, y + rTopRight,
            // right side
            'L', x + w, y + h - rBottomRight,
            // bottom right corner
            'C', x + w, y + h - rBottomRight / 2, x + w - rBottomRight / 2, y + h, x + w - rBottomRight, y + h,
            // bottom side
            'L', x + rBottomLeft, y + h,
            // bottom left corner
            'C', x + rBottomLeft / 2, y + h, x, y + h - rBottomLeft / 2, x, y + h - rBottomLeft,
            // left side
            'L', x, y + rTopLeft,
            // top left corner
            'C', x, y + rTopLeft / 2, x + rTopLeft / 2, y, x + rTopLeft, y,
            'Z'
        ];
    }
    SVGRenderer.prototype.symbols.topbutton = function (x, y, w, h, attr) {
        return selectiveRoundedRect(x - 1, y - 1, w, h, attr.r, attr.r, 0, 0);
    };
    SVGRenderer.prototype.symbols.bottombutton = function (x, y, w, h, attr) {
        return selectiveRoundedRect(x - 1, y - 1, w, h, 0, 0, attr.r, attr.r);
    };
    // The symbol callbacks are generated on the SVGRenderer object in all browsers. Even
    // VML browsers need this in order to generate shapes in export. Now share
    // them with the VMLRenderer.
    if (Renderer === VMLRenderer) {
        each(['topbutton', 'bottombutton'], function (shape) {
            VMLRenderer.prototype.symbols[shape] = SVGRenderer.prototype.symbols[shape];
        });
    }


    /**
     * A wrapper for Chart with all the default values for a Map
     */
    Highcharts.Map = function (options, callback) {

        var hiddenAxis = {
                endOnTick: false,
                gridLineWidth: 0,
                lineWidth: 0,
                minPadding: 0,
                maxPadding: 0,
                startOnTick: false,
                title: null,
                tickPositions: []
            },
            seriesOptions;

        /* For visual testing
        hiddenAxis.gridLineWidth = 1;
        hiddenAxis.gridZIndex = 10;
        hiddenAxis.tickPositions = undefined;
        // */

        // Don't merge the data
        seriesOptions = options.series;
        options.series = null;

        options = merge(
            {
                chart: {
                    panning: 'xy',
                    type: 'map'
                },
                xAxis: hiddenAxis,
                yAxis: merge(hiddenAxis, { reversed: true })
            },
            options, // user's options

            { // forced options
                chart: {
                    inverted: false,
                    alignTicks: false
                }
            }
        );

        options.series = seriesOptions;


        return new Chart(options, callback);
    };

}));
