/**
 * This is an experimental Highcharts module that draws long data series on a canvas
 * in order to increase performance of the initial load time and tooltip responsiveness.
 *
 * Compatible with HTML5 canvas compatible browsers (not IE < 9).
 *
 * Author: Torstein Honsi
 *
 * 
 * Development plan
 * - Column range.
 * - Heatmap.
 * - Treemap.
 * - Check how it works with Highstock and data grouping.
 * - Check inverted charts.
 * - Check reversed axes.
 * - Chart callback should be async after last series is drawn. (But not necessarily, we don't do
     that with initial series animation).
 * - Cache full-size image so we don't have to redraw on hide/show and zoom up. But k-d-tree still
 *   needs to be built.
 * - Test IE9 and IE10.
 * - Stacking is not perhaps not correct since it doesn't use the translation given in 
 *   the translate method. If this gets to complicated, a possible way out would be to 
 *   have a simplified renderCanvas method that simply draws the areaPath on a canvas.
 *
 * If this module is taken in as part of the core
 * - All the loading logic should be merged with core. Update styles in the core.
 * - Most of the method wraps should probably be added directly in parent methods.
 *
 * Notes for boost mode
 * - Area lines are not drawn
 * - Point markers are not drawn
 * - Zones and negativeColor don't work
 * - Columns are always one pixel wide. Don't set the threshold too low.
 *
 * Optimizing tips for users
 * - For scatter plots, use a marker.radius of 1 or less. It results in a rectangle being drawn, which is 
 *   considerably faster than a circle.
 * - Set extremes (min, max) explicitly on the axes in order for Highcharts to avoid computing extremes.
 * - Set enableMouseTracking to false on the series to improve total rendering time.
 * - The default threshold is set based on one series. If you have multiple, dense series, the combined
 *   number of points drawn gets higher, and you may want to set the threshold lower in order to 
 *   use optimizations.
 */

/* eslint indent: [2, 4] */
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = factory;
    } else {
        factory(Highcharts);
    }
}(function (H) {

    'use strict';

    var noop = function () {},
        Color = H.Color,
        Series = H.Series,
        seriesTypes = H.seriesTypes,
        each = H.each,
        extend = H.extend,
        addEvent = H.addEvent,
        fireEvent = H.fireEvent,
        merge = H.merge,
        pick = H.pick,
        wrap = H.wrap,
        plotOptions = H.getOptions().plotOptions,
        CHUNK_SIZE = 50000;

    function eachAsync(arr, fn, finalFunc, chunkSize, i) {
        i = i || 0;
        chunkSize = chunkSize || CHUNK_SIZE;
        each(arr.slice(i, i + chunkSize), fn);
        if (i + chunkSize < arr.length) {
            setTimeout(function () {
                eachAsync(arr, fn, finalFunc, chunkSize, i + chunkSize);
            });
        } else if (finalFunc) {
            finalFunc();
        }
    }

    // Set default options
    each(['area', 'arearange', 'column', 'line', 'scatter'], function (type) {
        if (plotOptions[type]) {
            plotOptions[type].boostThreshold = 5000;
        }
    });

    /**
     * Override a bunch of methods the same way. If the number of points is below the threshold,
     * run the original method. If not, check for a canvas version or do nothing.
     */
    each(['translate', 'generatePoints', 'drawTracker', 'drawPoints', 'render'], function (method) {
        function branch(proceed) {
            var letItPass = this.options.stacking && (method === 'translate' || method === 'generatePoints');
            if ((this.processedXData || this.options.data).length < (this.options.boostThreshold || Number.MAX_VALUE) ||
                    letItPass) {

                // Clear image
                if (method === 'render' && this.image) {
                    this.image.attr({ href: '' });
                    this.animate = null; // We're zooming in, don't run animation
                }

                proceed.call(this);

            // If a canvas version of the method exists, like renderCanvas(), run
            } else if (this[method + 'Canvas']) {

                this[method + 'Canvas']();
            }
        }
        wrap(Series.prototype, method, branch);

        // A special case for some types - its translate method is already wrapped
        if (method === 'translate') {
            if (seriesTypes.column) {
                wrap(seriesTypes.column.prototype, method, branch);
            }
            if (seriesTypes.arearange) {
                wrap(seriesTypes.arearange.prototype, method, branch);
            }
        }
    });

    /**
     * Do not compute extremes when min and max are set.
     * If we use this in the core, we can add the hook to hasExtremes to the methods directly.
     */
    wrap(Series.prototype, 'getExtremes', function (proceed) {
        if (!this.hasExtremes()) {
            proceed.apply(this, Array.prototype.slice.call(arguments, 1));
        }
    });
    wrap(Series.prototype, 'setData', function (proceed) {
        if (!this.hasExtremes(true)) {
            proceed.apply(this, Array.prototype.slice.call(arguments, 1));
        }
    });
    wrap(Series.prototype, 'processData', function (proceed) {
        if (!this.hasExtremes(true)) {
            proceed.apply(this, Array.prototype.slice.call(arguments, 1));
        }
    });


    H.extend(Series.prototype, {
        pointRange: 0,

        hasExtremes: function (checkX) {
            var options = this.options,
                data = options.data,
                xAxis = this.xAxis && this.xAxis.options,
                yAxis = this.yAxis && this.yAxis.options;
            return data.length > (options.boostThreshold || Number.MAX_VALUE) && typeof yAxis.min === 'number' && typeof yAxis.max === 'number' &&
                (!checkX || (typeof xAxis.min === 'number' && typeof xAxis.max === 'number'));
        },

        /**
         * If implemented in the core, parts of this can probably be shared with other similar
         * methods in Highcharts.
         */
        destroyGraphics: function () {
            var series = this,
                points = this.points,
                point,
                i;

            if (points) {
                for (i = 0; i < points.length; i = i + 1) {
                    point = points[i];
                    if (point && point.graphic) {
                        point.graphic = point.graphic.destroy();
                    }
                }
            }

            each(['graph', 'area', 'tracker'], function (prop) {
                if (series[prop]) {
                    series[prop] = series[prop].destroy();
                }
            });
        },

        /**
         * Create a hidden canvas to draw the graph on. The contents is later copied over 
         * to an SVG image element.
         */
        getContext: function () {
            var chart = this.chart,
                width = chart.plotWidth,
                height = chart.plotHeight,
                ctx = this.ctx,
                swapXY = function (proceed, x, y, a, b, c, d) {
                    proceed.call(this, y, x, a, b, c, d);
                };

            if (!this.canvas) {
                this.canvas = document.createElement('canvas');
                this.image = chart.renderer.image('', 0, 0, width, height).add(this.group);
                this.ctx = ctx = this.canvas.getContext('2d');
                if (chart.inverted) {
                    each(['moveTo', 'lineTo', 'rect', 'arc'], function (fn) {
                        wrap(ctx, fn, swapXY);
                    });
                }
            } else {
                ctx.clearRect(0, 0, width, height);
            }

            this.canvas.setAttribute('width', width);
            this.canvas.setAttribute('height', height);
            this.image.attr({
                width: width,
                height: height
            });

            return ctx;
        },

        /** 
         * Draw the canvas image inside an SVG image
         */
        canvasToSVG: function () {
            this.image.attr({ href: this.canvas.toDataURL('image/png') });
        },

        cvsLineTo: function (ctx, clientX, plotY) {
            ctx.lineTo(clientX, plotY);
        },

        renderCanvas: function () {
            var series = this,
                options = series.options,
                chart = series.chart,
                xAxis = this.xAxis,
                yAxis = this.yAxis,
                ctx,
                i,
                c = 0,
                xData = series.processedXData,
                yData = series.processedYData,
                rawData = options.data,
                xExtremes = xAxis.getExtremes(),
                xMin = xExtremes.min,
                xMax = xExtremes.max,
                yExtremes = yAxis.getExtremes(),
                yMin = yExtremes.min,
                yMax = yExtremes.max,
                pointTaken = {},
                lastClientX,
                sampling = !!series.sampling,
                points,
                r = options.marker && options.marker.radius,
                cvsDrawPoint = this.cvsDrawPoint,
                cvsLineTo = options.lineWidth ? this.cvsLineTo : false,
                cvsMarker = r <= 1 ? this.cvsMarkerSquare : this.cvsMarkerCircle,
                enableMouseTracking = options.enableMouseTracking !== false,
                lastPoint,
                threshold = options.threshold,
                yBottom = yAxis.getThreshold(threshold),
                hasThreshold = typeof threshold === 'number',
                translatedThreshold = yBottom,
                doFill = this.fill,
                isRange = series.pointArrayMap && series.pointArrayMap.join(',') === 'low,high',
                isStacked = !!options.stacking,
                cropStart = series.cropStart || 0,
                loadingOptions = chart.options.loading,
                requireSorting = series.requireSorting,
                wasNull,
                connectNulls = options.connectNulls,
                useRaw = !xData,
                minVal,
                maxVal,
                minI,
                maxI,
                fillColor = series.fillOpacity ?
                        new Color(series.color).setOpacity(pick(options.fillOpacity, 0.75)).get() :
                        series.color,
                stroke = function () {
                    if (doFill) {
                        ctx.fillStyle = fillColor;
                        ctx.fill();
                    } else {
                        ctx.strokeStyle = series.color;
                        ctx.lineWidth = options.lineWidth;
                        ctx.stroke();
                    }
                },
                drawPoint = function (clientX, plotY, yBottom) {
                    if (c === 0) {
                        ctx.beginPath();
                    }

                    if (wasNull) {
                        ctx.moveTo(clientX, plotY);
                    } else {
                        if (cvsDrawPoint) {
                            cvsDrawPoint(ctx, clientX, plotY, yBottom, lastPoint);
                        } else if (cvsLineTo) {
                            cvsLineTo(ctx, clientX, plotY);
                        } else if (cvsMarker) {
                            cvsMarker(ctx, clientX, plotY, r);
                        }
                    }

                    // We need to stroke the line for every 1000 pixels. It will crash the browser
                    // memory use if we stroke too infrequently.
                    c = c + 1;
                    if (c === 1000) {
                        stroke();
                        c = 0;
                    }

                    // Area charts need to keep track of the last point
                    lastPoint = {
                        clientX: clientX,
                        plotY: plotY,
                        yBottom: yBottom
                    };
                },

                addKDPoint = function (clientX, plotY, i) {

                    // The k-d tree requires series points. Reduce the amount of points, since the time to build the 
                    // tree increases exponentially.
                    if (enableMouseTracking && !pointTaken[clientX + ',' + plotY]) {
                        pointTaken[clientX + ',' + plotY] = true;

                        if (chart.inverted) {
                            clientX = xAxis.len - clientX;
                            plotY = yAxis.len - plotY;
                        }

                        points.push({
                            clientX: clientX,
                            plotX: clientX,
                            plotY: plotY,
                            i: cropStart + i
                        });
                    }
                };

            // If we are zooming out from SVG mode, destroy the graphics
            if (this.points || this.graph) {
                this.destroyGraphics();
            }

            // The group
            series.plotGroup(
                'group',
                'series',
                series.visible ? 'visible' : 'hidden',
                options.zIndex,
                chart.seriesGroup
            );

            series.getAttribs();
            series.markerGroup = series.group;
            addEvent(series, 'destroy', function () {
                series.markerGroup = null;
            });

            points = this.points = [];
            ctx = this.getContext();
            series.buildKDTree = noop; // Do not start building while drawing 

            // Display a loading indicator
            if (rawData.length > 99999) {
                chart.options.loading = merge(loadingOptions, {
                    labelStyle: {
                        backgroundColor: 'rgba(255,255,255,0.75)',
                        padding: '1em',
                        borderRadius: '0.5em'
                    },
                    style: {
                        backgroundColor: 'none',
                        opacity: 1
                    }
                });
                chart.showLoading('Drawing...');
                chart.options.loading = loadingOptions; // reset
                if (chart.loadingShown === true) {
                    chart.loadingShown = 1;
                } else {
                    chart.loadingShown = chart.loadingShown + 1;
                }
            }

            // Loop over the points
            i = 0;
            eachAsync(isStacked ? series.data : (xData || rawData), function (d) {

                var x,
                    y,
                    clientX,
                    plotY,
                    isNull,
                    low,
                    isYInside = true;

                if (useRaw) {
                    x = d[0];
                    y = d[1];
                } else {
                    x = d;
                    y = yData[i];
                }

                // Resolve low and high for range series
                if (isRange) {
                    if (useRaw) {
                        y = d.slice(1, 3);
                    }
                    low = y[0];
                    y = y[1];
                } else if (isStacked) {
                    x = d.x;
                    y = d.stackY;
                    low = y - d.y;
                }

                isNull = y === null;

                // Optimize for scatter zooming
                if (!requireSorting) {
                    isYInside = y >= yMin && y <= yMax;
                }

                if (!isNull && x >= xMin && x <= xMax && isYInside) {

                    clientX = Math.round(xAxis.toPixels(x, true));

                    if (sampling) {
                        if (minI === undefined || clientX === lastClientX) {
                            if (!isRange) {
                                low = y;
                            }
                            if (maxI === undefined || y > maxVal) {
                                maxVal = y;
                                maxI = i;
                            }
                            if (minI === undefined || low < minVal) {
                                minVal = low;
                                minI = i;
                            }

                        }
                        if (clientX !== lastClientX) { // Add points and reset
                            if (minI !== undefined) { // then maxI is also a number
                                plotY = yAxis.toPixels(maxVal, true);
                                yBottom = yAxis.toPixels(minVal, true);
                                drawPoint(
                                    clientX,
                                    hasThreshold ? Math.min(plotY, translatedThreshold) : plotY,
                                    hasThreshold ? Math.max(yBottom, translatedThreshold) : yBottom
                                );
                                addKDPoint(clientX, plotY, maxI);
                                if (yBottom !== plotY) {
                                    addKDPoint(clientX, yBottom, minI);
                                }
                            }


                            minI = maxI = undefined;
                            lastClientX = clientX;
                        }
                    } else {
                        plotY = Math.round(yAxis.toPixels(y, true));
                        drawPoint(clientX, plotY, yBottom);
                        addKDPoint(clientX, plotY, i);
                    }
                }
                wasNull = isNull && !connectNulls;

                i = i + 1;

                if (i % CHUNK_SIZE === 0) {
                    series.canvasToSVG();
                }

            }, function () {

                var loadingDiv = chart.loadingDiv,
                    loadingShown = +chart.loadingShown;

                stroke();
                series.canvasToSVG();

                fireEvent(series, 'renderedCanvas');

                // Do not use chart.hideLoading, as it runs JS animation and will be blocked by buildKDTree.
                // CSS animation looks good, but then it must be deleted in timeout. If we add the module to core,
                // change hideLoading so we can skip this block.
                if (loadingShown === 1) {
                    extend(loadingDiv.style, {
                        transition: 'opacity 250ms',
                        opacity: 0
                    });

                    chart.loadingShown = false;
                    setTimeout(function () {
                        if (loadingDiv.parentNode) { // In exporting it is falsy
                            loadingDiv.parentNode.removeChild(loadingDiv);
                        }
                        chart.loadingDiv = chart.loadingSpan = null;
                    }, 250);
                }
                if (loadingShown) {
                    chart.loadingShown = loadingShown - 1;
                }

                // Pass tests in Pointer. 
                // Replace this with a single property, and replace when zooming in
                // below boostThreshold.
                series.directTouch = false;
                series.options.stickyTracking = true;

                delete series.buildKDTree; // Go back to prototype, ready to build
                series.buildKDTree();

             // Don't do async on export, the exportChart, getSVGForExport and getSVG methods are not chained for it.
            }, chart.renderer.forExport ? Number.MAX_VALUE : undefined);
        }
    });

    seriesTypes.scatter.prototype.cvsMarkerCircle = function (ctx, clientX, plotY, r) {
        ctx.moveTo(clientX, plotY);
        ctx.arc(clientX, plotY, r, 0, 2 * Math.PI, false);
    };

    // Rect is twice as fast as arc, should be used for small markers
    seriesTypes.scatter.prototype.cvsMarkerSquare = function (ctx, clientX, plotY, r) {
        ctx.moveTo(clientX, plotY);
        ctx.rect(clientX - r, plotY - r, r * 2, r * 2);
    };
    seriesTypes.scatter.prototype.fill = true;

    extend(seriesTypes.area.prototype, {
        cvsDrawPoint: function (ctx, clientX, plotY, yBottom, lastPoint) {
            if (lastPoint && clientX !== lastPoint.clientX) {
                ctx.moveTo(lastPoint.clientX, lastPoint.yBottom);
                ctx.lineTo(lastPoint.clientX, lastPoint.plotY);
                ctx.lineTo(clientX, plotY);
                ctx.lineTo(clientX, yBottom);
            }
        },
        fill: true,
        fillOpacity: true,
        sampling: true
    });

    extend(seriesTypes.column.prototype, {
        cvsDrawPoint: function (ctx, clientX, plotY, yBottom) {
            ctx.rect(clientX - 1, plotY, 1, yBottom - plotY);
        },
        fill: true,
        sampling: true
    });

    /**
     * Return a full Point object based on the index. The boost module uses stripped point objects
     * for performance reasons.
     * @param   {Number} boostPoint A stripped-down point object
     * @returns {Object}   A Point object as per http://api.highcharts.com/highcharts#Point
     */
    Series.prototype.getPoint = function (boostPoint) {
        var point = boostPoint;

        if (boostPoint && !(boostPoint instanceof this.pointClass)) {
            point = (new this.pointClass()).init(this, this.options.data[boostPoint.i]);
            point.dist = boostPoint.dist;
            point.category = point.x;
            point.plotX = boostPoint.plotX;
            point.plotY = boostPoint.plotY;
        }

        return point;
    };

    /**
     * Return a point instance from the k-d-tree
     */
    wrap(Series.prototype, 'searchPoint', function (proceed) {
        return this.getPoint(
            proceed.apply(this, [].slice.call(arguments, 1))
        );
    });
}));
