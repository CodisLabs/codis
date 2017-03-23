/**
 * EXPERIMENTAL Highcharts module to place labels next to a series in a natural position.
 *
 * TODO:
 * - add column support (box collision detection, same as above)
 * - other series types, area etc.
 * - avoid data labels, when data labels above, show series label below.
 * - add options (enabled, style, connector, format, formatter)
 * - connectors: Make a separate shape with anchors to use as label
 * - do labels in a timeout since they don't interfere with others
 * 
 * http://jsfiddle.net/highcharts/L2u9rpwr/
 * http://jsfiddle.net/highcharts/y5A37/
 * http://jsfiddle.net/highcharts/264Nm/
 * http://jsfiddle.net/highcharts/y5A37/
 */
/* eslint indent: [2, 4] */
(function (factory) {
    if (typeof module === 'object' && module.exports) {
        module.exports = factory;
    } else {
        factory(Highcharts);
    }
}(function (H) {

    var labelDistance = 3,
        wrap = H.wrap,
        each = H.each,
        Series = H.Series,
        Chart = H.Chart;

    /**
     * Counter-clockwise, part of the fast line intersection logic
     */
    function ccw(x1, y1, x2, y2, x3, y3) {
        var cw = ((y3 - y1) * (x2 - x1)) - ((y2 - y1) * (x3 - x1));
        return cw > 0 ? true : cw < 0 ? false : true;
    }

    /**
     * Detect if two lines intersect
     */
    function intersectLine(x1, y1, x2, y2, x3, y3, x4, y4) {
        return ccw(x1, y1, x3, y3, x4, y4) !== ccw(x2, y2, x3, y3, x4, y4) &&
            ccw(x1, y1, x2, y2, x3, y3) !== ccw(x1, y1, x2, y2, x4, y4);
    }

    /**
     * Detect if a box intersects with a line
     */
    function boxIntersectLine(x, y, w, h, x1, y1, x2, y2) {
        return (
            intersectLine(x, y, x + w, y,         x1, y1, x2, y2) || // top of label
            intersectLine(x + w, y, x + w, y + h, x1, y1, x2, y2) || // right of label
            intersectLine(x, y + h, x + w, y + h, x1, y1, x2, y2) || // bottom of label
            intersectLine(x, y, x, y + h,         x1, y1, x2, y2)    // left of label
        );
    }

    /**
     * Points to avoid. In addition to actual data points, the label should avoid
     * interpolated positions.
     */
    Series.prototype.getPointsOnGraph = function () {
        var distance = 16,
            points = this.points,
            point,
            interpolated = [],
            i,
            deltaX,
            deltaY,
            delta,
            last,
            len,
            n,
            j,
            d,
            node = this.graph.element;

        // For splines, get the point at length (possible caveat: peaks are not correctly detected)
        if (this.getPointSpline && node.getPointAtLength) {
            // If it is animating towards a path definition, use that briefly, and reset
            if (this.graph.toD) {
                d = this.graph.attr('d');
                this.graph.attr({ d: this.graph.toD });
            }
            len = node.getTotalLength();
            for (i = 0; i < len; i += distance) {
                point = node.getPointAtLength(i);
                interpolated.push({
                    plotX: point.x,
                    plotY: point.y
                });
            }
            if (d) {
                this.graph.attr({ d: d });
            }
            // Last point
            interpolated.push(points[points.length - 1]);

        // Interpolate
        } else {
            len = points.length;
            for (i = 0; i < len; i += 1) {

                // Add interpolated points
                if (i > 0) {
                    deltaX = Math.abs(points[i].plotX - points[i - 1].plotX);
                    deltaY = Math.abs(points[i].plotY - points[i - 1].plotY);
                    delta = Math.max(deltaX, deltaY);
                    if (delta > distance) {

                        n = Math.ceil(delta / distance);

                        for (j = 1; j < n; j += 1) {
                            interpolated.push({
                                plotX: points[i - 1].plotX + (points[i].plotX - points[i - 1].plotX) * (j / n),
                                plotY: points[i - 1].plotY + (points[i].plotY - points[i - 1].plotY) * (j / n)
                            });
                        }
                    }
                }

                // Add the real point if not too close to the previous
                if (typeof points[i].plotY === 'number') {
                    last = interpolated[interpolated.length - 1];
                    if (last) {
                        delta = Math.max(
                            Math.abs(points[i].plotX - last.plotX),
                            Math.abs(points[i].plotY - last.plotY)
                        );
                    }
                    if (!last || delta > distance / 2) {
                        interpolated.push(points[i]);
                    }
                }
            }
        }
        return interpolated;
    };

    /**
     * Check whether a proposed label position is clear of other elements
     */
    Series.prototype.checkClearPoint = function (x, y, bBox, checkDistance) {
        var distToOthersSquared = Number.MAX_VALUE, // distance to other graphs
            distToPointSquared = Number.MAX_VALUE,
            dist,
            connectorPoint,
            connectorEnabled = true, // make part of the options set
            chart = this.chart,
            series,
            points,
            leastDistance = 16,
            withinRange,
            i,
            j;

        function intersectRect(r1, r2) {
            return !(r2.left > r1.right ||
                r2.right < r1.left ||
                r2.top > r1.bottom ||
                r2.bottom < r1.top);
        }

        /**
         * Get the weight in order to determine the ideal position. Larger distance to
         * other series gives more weight. Smaller distance to the actual point (connector points only)
         * gives more weight.
         */
        function getWeight(distToOthersSquared, distToPointSquared) {
            return distToOthersSquared - distToPointSquared;
        }

        // First check for collision with existing labels
        for (i = 0; i < chart.boxesToAvoid.length; i += 1) {
            if (intersectRect(chart.boxesToAvoid[i], {
                left: x,
                right: x + bBox.width,
                top: y,
                bottom: y + bBox.height
            })) {
                return false;
            }
        }

        // For each position, check if the lines around the label intersect with any of the 
        // graphs
        for (i = 0; i < chart.series.length; i += 1) {
            series = chart.series[i];
            if (series.visible) {
                points = series.interpolatedPoints;
                for (j = 1; j < points.length; j += 1) {
                    // If any of the box sides intersect with the line, return
                    if (boxIntersectLine(
                            x,
                            y,
                            bBox.width,
                            bBox.height,
                            points[j - 1].plotX,
                            points[j - 1].plotY,
                            points[j].plotX,
                            points[j].plotY
                        )) {
                        return false;
                    }

                    // But if it is too far away (a padded box doesn't intersect), also return
                    if (this === series && !withinRange && checkDistance) {
                        withinRange = boxIntersectLine(
                            x - leastDistance,
                            y - leastDistance,
                            bBox.width + 2 * leastDistance,
                            bBox.height + 2 * leastDistance,
                            points[j - 1].plotX,
                            points[j - 1].plotY,
                            points[j].plotX,
                            points[j].plotY
                        );
                    }

                    // Find the squared distance from the center of the label
                    if (this !== series) {
                        distToOthersSquared = Math.min(
                            distToOthersSquared,
                            Math.pow(x + bBox.width / 2 - points[j].plotX, 2) + Math.pow(y + bBox.height / 2 - points[j].plotY, 2),
                            Math.pow(x - points[j].plotX, 2) + Math.pow(y - points[j].plotY, 2),
                            Math.pow(x + bBox.width - points[j].plotX, 2) + Math.pow(y - points[j].plotY, 2),
                            Math.pow(x + bBox.width - points[j].plotX, 2) + Math.pow(y + bBox.height - points[j].plotY, 2),
                            Math.pow(x - points[j].plotX, 2) + Math.pow(y + bBox.height - points[j].plotY, 2)
                        );
                    }
                }

                // Do we need a connector? 
                if (connectorEnabled && this === series && checkDistance && !withinRange) {
                    for (j = 1; j < points.length; j += 1) {
                        dist = Math.min(
                            Math.pow(x + bBox.width / 2 - points[j].plotX, 2) + Math.pow(y + bBox.height / 2 - points[j].plotY, 2),
                            Math.pow(x - points[j].plotX, 2) + Math.pow(y - points[j].plotY, 2),
                            Math.pow(x + bBox.width - points[j].plotX, 2) + Math.pow(y - points[j].plotY, 2),
                            Math.pow(x + bBox.width - points[j].plotX, 2) + Math.pow(y + bBox.height - points[j].plotY, 2),
                            Math.pow(x - points[j].plotX, 2) + Math.pow(y + bBox.height - points[j].plotY, 2)
                        );
                        if (dist < distToPointSquared) {
                            distToPointSquared = dist;
                            connectorPoint = points[j];
                        }
                    }
                    withinRange = true;
                }
            }
        }

        return !checkDistance || withinRange ? {
            x: x,
            y: y,
            weight: getWeight(distToOthersSquared, connectorPoint ? distToPointSquared : 0),
            connectorPoint: connectorPoint
        } : false;

    };


    function drawLabels(proceed) {

        proceed.call(this);

        //console.time('labelBySeries');

        //this.buildTreeToAvoid();
        this.boxesToAvoid = [];

        // Build the interpolated points
        each(this.series, function (series) {
            if (series.visible && series.graph) {
                series.interpolatedPoints = series.getPointsOnGraph();
            }
        });

        each(this.series, function (series) {
            var chart = series.chart,
                bBox,
                x,
                y,
                results = [],
                clearPoint,
                i,
                best,
                points = series.interpolatedPoints;

            if (series.visible) {

                if (!series.labelBySeries) {
                    series.labelBySeries = chart.renderer.label(series.name, 0, -9999)
                        .css({
                            color: series.color,
                            fontWeight: 'bold'
                        })
                        .attr({
                            padding: 0
                        })
                        .add(series.group);
                    /*series.labelConnector = chart.renderer.path([]).attr({
                        stroke: series.color,
                        'stroke-width': 1
                    })
                    .add(series.group);*/
                }

                bBox = series.labelBySeries.getBBox();
                bBox.width = Math.round(bBox.width);

                // Ideal positions are centered above or below a point on right side of chart
                for (i = points.length - 1; i > 0; i -= 1) {

                    // Right - up
                    x = points[i].plotX + labelDistance;
                    y = points[i].plotY - bBox.height - labelDistance;
                    if (x > 0 && x <= chart.plotWidth - bBox.width && y >= 0 && y <= chart.plotHeight - bBox.height) {
                        best = series.checkClearPoint(
                            x,
                            y,
                            bBox
                        );
                    }
                    if (best) {
                        results.push(best);
                        //break;
                    }

                    // Right - down
                    x = points[i].plotX + labelDistance;
                    y = points[i].plotY + labelDistance;
                    if (x > 0 && x <= chart.plotWidth - bBox.width && y >= 0 && y <= chart.plotHeight - bBox.height) {
                        best = series.checkClearPoint(
                            x,
                            y,
                            bBox
                        );
                    }
                    if (best) {
                        results.push(best);
                        //break;
                    }

                    // Left - down
                    x = points[i].plotX - bBox.width - labelDistance;
                    y = points[i].plotY + labelDistance;
                    if (x > 0 && x <= chart.plotWidth - bBox.width && y >= 0 && y <= chart.plotHeight - bBox.height) {
                        best = series.checkClearPoint(
                            x,
                            y,
                            bBox
                        );
                    }
                    if (best) {
                        results.push(best);
                        //break;
                    }

                    // Left - up
                    x = points[i].plotX - bBox.width - labelDistance;
                    y = points[i].plotY - bBox.height - labelDistance;
                    if (x > 0 && x <= chart.plotWidth - bBox.width && y >= 0 && y <= chart.plotHeight - bBox.height) {
                        best = series.checkClearPoint(
                            x,
                            y,
                            bBox
                        );
                    }
                    if (best) {
                        results.push(best);
                        //break;
                    }

                }

                // Brute force, try all positions on the chart in a 16x16 grid
                if (!results.length) {
                    for (x = chart.plotWidth - bBox.width; x >= 0; x -= 16) {
                        for (y = 0; y < chart.plotHeight - bBox.height; y += 16) {
                            clearPoint = series.checkClearPoint(x, y, bBox, true);
                            if (clearPoint) {
                                results.push(clearPoint);
                            }
                        }
                    }
                }

                if (results.length) {

                    results.sort(function (a, b) {
                        return b.weight - a.weight;
                    });
                    //results = results.reverse();
                    best = results[0];

                    chart.boxesToAvoid.push({
                        left: best.x,
                        right: best.x + bBox.width,
                        top: best.y,
                        bottom: best.y + bBox.height
                    });
                    series.labelBySeries.attr({
                        x: best.x,
                        y: best.y
                    });
                } else if (series.labelBySeries) {
                    series.labelBySeries = series.labelBySeries.destroy();
                }
            }
        });
        //console.timeEnd('labelBySeries');

    }
    wrap(Chart.prototype, 'render', drawLabels);
    wrap(Chart.prototype, 'redraw', drawLabels);

}));
