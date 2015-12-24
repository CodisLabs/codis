/**
 * Highcharts Drilldown module
 * 
 * Author: Torstein Honsi
 * License: www.highcharts.com/license
 *
 */

(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (H) {

	'use strict';

	var noop = function () {},
		defaultOptions = H.getOptions(),
		each = H.each,
		extend = H.extend,
		format = H.format,
		pick = H.pick,
		wrap = H.wrap,
		Chart = H.Chart,
		seriesTypes = H.seriesTypes,
		PieSeries = seriesTypes.pie,
		ColumnSeries = seriesTypes.column,
		Tick = H.Tick,
		fireEvent = H.fireEvent,
		inArray = H.inArray,
		ddSeriesId = 1;

	// Utilities
	/*
	 * Return an intermediate color between two colors, according to pos where 0
	 * is the from color and 1 is the to color. This method is copied from ColorAxis.js
	 * and should always be kept updated, until we get AMD support.
	 */
	function tweenColors(from, to, pos) {
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
	}
	/**
	 * Handle animation of the color attributes directly
	 */
	each(['fill', 'stroke'], function (prop) {
		H.addAnimSetter(prop, function (fx) {
			fx.elem.attr(prop, tweenColors(H.Color(fx.start), H.Color(fx.end), fx.pos));
		});
	});

	// Add language
	extend(defaultOptions.lang, {
		drillUpText: '◁ Back to {series.name}'
	});
	defaultOptions.drilldown = {
		activeAxisLabelStyle: {
			cursor: 'pointer',
			color: '#0d233a',
			fontWeight: 'bold',
			textDecoration: 'underline'			
		},
		activeDataLabelStyle: {
			cursor: 'pointer',
			color: '#0d233a',
			fontWeight: 'bold',
			textDecoration: 'underline'			
		},
		animation: {
			duration: 500
		},
		drillUpButton: {
			position: { 
				align: 'right',
				x: -10,
				y: 10
			}
			// relativeTo: 'plotBox'
			// theme
		}
	};	

	/**
	 * A general fadeIn method
	 */
	H.SVGRenderer.prototype.Element.prototype.fadeIn = function (animation) {
		this
		.attr({
			opacity: 0.1,
			visibility: 'inherit'
		})
		.animate({
			opacity: pick(this.newOpacity, 1) // newOpacity used in maps
		}, animation || {
			duration: 250
		});
	};

	Chart.prototype.addSeriesAsDrilldown = function (point, ddOptions) {
		this.addSingleSeriesAsDrilldown(point, ddOptions);
		this.applyDrilldown();
	};
	Chart.prototype.addSingleSeriesAsDrilldown = function (point, ddOptions) {
		var oldSeries = point.series,
			xAxis = oldSeries.xAxis,
			yAxis = oldSeries.yAxis,
			newSeries,
			color = point.color || oldSeries.color,
			pointIndex,
			levelSeries = [],
			levelSeriesOptions = [],
			level,
			levelNumber,
			last;

		if (!this.drilldownLevels) {
			this.drilldownLevels = [];
		}
		
		levelNumber = oldSeries.options._levelNumber || 0;

		// See if we can reuse the registered series from last run
		last = this.drilldownLevels[this.drilldownLevels.length - 1];
		if (last && last.levelNumber !== levelNumber) {
			last = undefined;
		}
		
			
		ddOptions = extend({
			color: color,
			_ddSeriesId: ddSeriesId++
		}, ddOptions);
		pointIndex = inArray(point, oldSeries.points);

		// Record options for all current series
		each(oldSeries.chart.series, function (series) {
			if (series.xAxis === xAxis && !series.isDrilling) {
				series.options._ddSeriesId = series.options._ddSeriesId || ddSeriesId++;
				series.options._colorIndex = series.userOptions._colorIndex;
				series.options._levelNumber = series.options._levelNumber || levelNumber; // #3182

				if (last) {
					levelSeries = last.levelSeries;
					levelSeriesOptions = last.levelSeriesOptions;
				} else {
					levelSeries.push(series);
					levelSeriesOptions.push(series.options);
				}
			}
		});

		// Add a record of properties for each drilldown level
		level = {
			levelNumber: levelNumber,
			seriesOptions: oldSeries.options,
			levelSeriesOptions: levelSeriesOptions,
			levelSeries: levelSeries,
			shapeArgs: point.shapeArgs,
			bBox: point.graphic ? point.graphic.getBBox() : {}, // no graphic in line series with markers disabled
			color: color,
			lowerSeriesOptions: ddOptions,
			pointOptions: oldSeries.options.data[pointIndex],
			pointIndex: pointIndex,
			oldExtremes: {
				xMin: xAxis && xAxis.userMin,
				xMax: xAxis && xAxis.userMax,
				yMin: yAxis && yAxis.userMin,
				yMax: yAxis && yAxis.userMax
			}
		};

		// Push it to the lookup array
		this.drilldownLevels.push(level);

		newSeries = level.lowerSeries = this.addSeries(ddOptions, false);
		newSeries.options._levelNumber = levelNumber + 1;
		if (xAxis) {
			xAxis.oldPos = xAxis.pos;
			xAxis.userMin = xAxis.userMax = null;
			yAxis.userMin = yAxis.userMax = null;
		}

		// Run fancy cross-animation on supported and equal types
		if (oldSeries.type === newSeries.type) {
			newSeries.animate = newSeries.animateDrilldown || noop;
			newSeries.options.animation = true;
		}
	};

	Chart.prototype.applyDrilldown = function () {
		var drilldownLevels = this.drilldownLevels, 
			levelToRemove;
		
		if (drilldownLevels && drilldownLevels.length > 0) { // #3352, async loading
			levelToRemove = drilldownLevels[drilldownLevels.length - 1].levelNumber;
			each(this.drilldownLevels, function (level) {
				if (level.levelNumber === levelToRemove) {
					each(level.levelSeries, function (series) {
						if (series.options && series.options._levelNumber === levelToRemove) { // Not removed, not added as part of a multi-series drilldown
							series.remove(false);
						}
					});
				}
			});
		}
		
		this.redraw();
		this.showDrillUpButton();
	};

	Chart.prototype.getDrilldownBackText = function () {
		var drilldownLevels = this.drilldownLevels,
			lastLevel;
		if (drilldownLevels && drilldownLevels.length > 0) { // #3352, async loading
			lastLevel = drilldownLevels[drilldownLevels.length - 1];
			lastLevel.series = lastLevel.seriesOptions;
			return format(this.options.lang.drillUpText, lastLevel);
		}

	};

	Chart.prototype.showDrillUpButton = function () {
		var chart = this,
			backText = this.getDrilldownBackText(),
			buttonOptions = chart.options.drilldown.drillUpButton,
			attr,
			states;
			

		if (!this.drillUpButton) {
			attr = buttonOptions.theme;
			states = attr && attr.states;
						
			this.drillUpButton = this.renderer.button(
				backText,
				null,
				null,
				function () {
					chart.drillUp(); 
				},
				attr, 
				states && states.hover,
				states && states.select
			)
			.attr({
				align: buttonOptions.position.align,
				zIndex: 9
			})
			.add()
			.align(buttonOptions.position, false, buttonOptions.relativeTo || 'plotBox');
		} else {
			this.drillUpButton.attr({
				text: backText
			})
			.align();
		}
	};

	Chart.prototype.drillUp = function () {
		var chart = this,
			drilldownLevels = chart.drilldownLevels,
			levelNumber = drilldownLevels[drilldownLevels.length - 1].levelNumber,
			i = drilldownLevels.length,
			chartSeries = chart.series,
			seriesI,
			level,
			oldSeries,
			newSeries,
			oldExtremes,
			addSeries = function (seriesOptions) {
				var addedSeries;
				each(chartSeries, function (series) {
					if (series.options._ddSeriesId === seriesOptions._ddSeriesId) {
						addedSeries = series;
					}
				});

				addedSeries = addedSeries || chart.addSeries(seriesOptions, false);
				if (addedSeries.type === oldSeries.type && addedSeries.animateDrillupTo) {
					addedSeries.animate = addedSeries.animateDrillupTo;
				}
				if (seriesOptions === level.seriesOptions) {
					newSeries = addedSeries;
				}
			};

		while (i--) {

			level = drilldownLevels[i];
			if (level.levelNumber === levelNumber) {
				drilldownLevels.pop();
				
				// Get the lower series by reference or id
				oldSeries = level.lowerSeries;
				if (!oldSeries.chart) {  // #2786
					seriesI = chartSeries.length; // #2919
					while (seriesI--) {
						if (chartSeries[seriesI].options.id === level.lowerSeriesOptions.id && 
								chartSeries[seriesI].options._levelNumber === levelNumber + 1) { // #3867
							oldSeries = chartSeries[seriesI];
							break;
						}
					}
				}
				oldSeries.xData = []; // Overcome problems with minRange (#2898)

				each(level.levelSeriesOptions, addSeries);
				
				fireEvent(chart, 'drillup', { seriesOptions: level.seriesOptions });

				if (newSeries.type === oldSeries.type) {
					newSeries.drilldownLevel = level;
					newSeries.options.animation = chart.options.drilldown.animation;

					if (oldSeries.animateDrillupFrom && oldSeries.chart) { // #2919
						oldSeries.animateDrillupFrom(level);
					}
				}
				newSeries.options._levelNumber = levelNumber;
				
				oldSeries.remove(false);

				// Reset the zoom level of the upper series
				if (newSeries.xAxis) {
					oldExtremes = level.oldExtremes;
					newSeries.xAxis.setExtremes(oldExtremes.xMin, oldExtremes.xMax, false);
					newSeries.yAxis.setExtremes(oldExtremes.yMin, oldExtremes.yMax, false);
				}
			}
		}

		this.redraw();

		if (this.drilldownLevels.length === 0) {
			this.drillUpButton = this.drillUpButton.destroy();
		} else {
			this.drillUpButton.attr({
				text: this.getDrilldownBackText()
			})
			.align();
		}

		this.ddDupes.length = []; // #3315
	};


	ColumnSeries.prototype.supportsDrilldown = true;
	
	/**
	 * When drilling up, keep the upper series invisible until the lower series has
	 * moved into place
	 */
	ColumnSeries.prototype.animateDrillupTo = function (init) {
		if (!init) {
			var newSeries = this,
				level = newSeries.drilldownLevel;

			each(this.points, function (point) {
				if (point.graphic) { // #3407
					point.graphic.hide();
				}
				if (point.dataLabel) {
					point.dataLabel.hide();
				}
				if (point.connector) {
					point.connector.hide();
				}
			});


			// Do dummy animation on first point to get to complete
			setTimeout(function () {
				if (newSeries.points) { // May be destroyed in the meantime, #3389
					each(newSeries.points, function (point, i) {  
						// Fade in other points			  
						var verb = i === (level && level.pointIndex) ? 'show' : 'fadeIn',
							inherit = verb === 'show' ? true : undefined;
						if (point.graphic) { // #3407
							point.graphic[verb](inherit);
						}
						if (point.dataLabel) {
							point.dataLabel[verb](inherit);
						}
						if (point.connector) {
							point.connector[verb](inherit);
						}
					});
				}
			}, Math.max(this.chart.options.drilldown.animation.duration - 50, 0));

			// Reset
			this.animate = noop;
		}

	};
	
	ColumnSeries.prototype.animateDrilldown = function (init) {
		var series = this,
			drilldownLevels = this.chart.drilldownLevels,
			animateFrom,
			animationOptions = this.chart.options.drilldown.animation,
			xAxis = this.xAxis;
			
		if (!init) {
			each(drilldownLevels, function (level) {
				if (series.options._ddSeriesId === level.lowerSeriesOptions._ddSeriesId) {
					animateFrom = level.shapeArgs;
					animateFrom.fill = level.color;
				}
			});

			animateFrom.x += (pick(xAxis.oldPos, xAxis.pos) - xAxis.pos);

			each(this.points, function (point) {
				if (point.graphic) {
					point.graphic
						.attr(animateFrom)
						.animate(
							extend(point.shapeArgs, { fill: point.color }), 
							animationOptions
						);
				}
				if (point.dataLabel) {
					point.dataLabel.fadeIn(animationOptions);
				}
			});
			this.animate = null;
		}
		
	};

	/**
	 * When drilling up, pull out the individual point graphics from the lower series
	 * and animate them into the origin point in the upper series.
	 */
	ColumnSeries.prototype.animateDrillupFrom = function (level) {
		var animationOptions = this.chart.options.drilldown.animation,
			group = this.group,
			series = this;

		// Cancel mouse events on the series group (#2787)
		each(series.trackerGroups, function (key) {
			if (series[key]) { // we don't always have dataLabelsGroup
				series[key].on('mouseover');
			}
		});
			

		delete this.group;
		each(this.points, function (point) {
			var graphic = point.graphic,
				complete = function () {
					graphic.destroy();
					if (group) {
						group = group.destroy();
					}
				};

			if (graphic) {
			
				delete point.graphic;

				if (animationOptions) {
					graphic.animate(
						extend(level.shapeArgs, { fill: level.color }),
						H.merge(animationOptions, { complete: complete })
					);
				} else {
					graphic.attr(level.shapeArgs);
					complete();
				}
			}
		});
	};

	if (PieSeries) {
		extend(PieSeries.prototype, {
			supportsDrilldown: true,
			animateDrillupTo: ColumnSeries.prototype.animateDrillupTo,
			animateDrillupFrom: ColumnSeries.prototype.animateDrillupFrom,

			animateDrilldown: function (init) {
				var level = this.chart.drilldownLevels[this.chart.drilldownLevels.length - 1],
					animationOptions = this.chart.options.drilldown.animation,
					animateFrom = level.shapeArgs,
					start = animateFrom.start,
					angle = animateFrom.end - start,
					startAngle = angle / this.points.length;

				if (!init) {
					each(this.points, function (point, i) {
						point.graphic
							.attr(H.merge(animateFrom, {
								start: start + i * startAngle,
								end: start + (i + 1) * startAngle,
								fill: level.color
							}))[animationOptions ? 'animate' : 'attr'](
								extend(point.shapeArgs, { fill: point.color }), 
								animationOptions
							);
					});
					this.animate = null;
				}
			}
		});
	}
	
	H.Point.prototype.doDrilldown = function (_holdRedraw, category) {
		var series = this.series,
			chart = series.chart,
			drilldown = chart.options.drilldown,
			i = (drilldown.series || []).length,
			seriesOptions;

		if (!chart.ddDupes) {
			chart.ddDupes = [];
		}
		
		while (i-- && !seriesOptions) {
			if (drilldown.series[i].id === this.drilldown && inArray(this.drilldown, chart.ddDupes) === -1) {
				seriesOptions = drilldown.series[i];
				chart.ddDupes.push(this.drilldown);
			}
		}

		// Fire the event. If seriesOptions is undefined, the implementer can check for 
		// seriesOptions, and call addSeriesAsDrilldown async if necessary.
		fireEvent(chart, 'drilldown', { 
			point: this,
			seriesOptions: seriesOptions,
			category: category,
			points: category !== undefined && this.series.xAxis.ddPoints[category].slice(0)
		});
		
		if (seriesOptions) {
			if (_holdRedraw) {
				chart.addSingleSeriesAsDrilldown(this, seriesOptions);
			} else {
				chart.addSeriesAsDrilldown(this, seriesOptions);
			}
		}
	};

	/**
	 * Drill down to a given category. This is the same as clicking on an axis label.
	 */
	H.Axis.prototype.drilldownCategory = function (x) {
		var key,
			point,
			ddPointsX = this.ddPoints[x];
		for (key in ddPointsX) {
			point = ddPointsX[key];
			if (point && point.series && point.series.visible && point.doDrilldown) { // #3197
				point.doDrilldown(true, x);
			}
		}
		this.chart.applyDrilldown();
	};

	/**
	 * Create and return a collection of points associated with the X position. Reset it for each level.
	 */	
	H.Axis.prototype.getDDPoints = function (x, levelNumber) {
		var ddPoints = this.ddPoints;
		if (!ddPoints) {
			this.ddPoints = ddPoints = {};
		}
		if (!ddPoints[x]) {
			ddPoints[x] = [];
		}
		if (ddPoints[x].levelNumber !== levelNumber) {
			ddPoints[x].length = 0; // reset
		}
		return ddPoints[x];
	};


	/**
	 * Make a tick label drillable, or remove drilling on update
	 */
	Tick.prototype.drillable = function () {
		var pos = this.pos,
			label = this.label,
			axis = this.axis,
			ddPointsX = axis.ddPoints && axis.ddPoints[pos];

		if (label && ddPointsX && ddPointsX.length) {
			if (!label.basicStyles) {
				label.basicStyles = H.merge(label.styles);
			}
			label
				.addClass('highcharts-drilldown-axis-label')
				.css(axis.chart.options.drilldown.activeAxisLabelStyle)
				.on('click', function () {
					axis.drilldownCategory(pos);
				});

		} else if (label && label.basicStyles) {
			label.styles = {}; // reset for full overwrite of styles
			label.css(label.basicStyles);
			label.on('click', null); // #3806			
		}
	};

	/**
	 * Always keep the drillability updated (#3951)
	 */
	wrap(Tick.prototype, 'addLabel', function (proceed) {
		proceed.call(this);
		this.drillable();
	});
	

	/**
	 * On initialization of each point, identify its label and make it clickable. Also, provide a
	 * list of points associated to that label.
	 */
	wrap(H.Point.prototype, 'init', function (proceed, series, options, x) {
		var point = proceed.call(this, series, options, x),
			xAxis = series.xAxis,
			tick = xAxis && xAxis.ticks[x],
			ddPointsX = xAxis && xAxis.getDDPoints(x, series.options._levelNumber);

		if (point.drilldown) {
			
			// Add the click event to the point 
			H.addEvent(point, 'click', function () {
				if (series.xAxis && series.chart.options.drilldown.allowPointDrilldown === false) {
					series.xAxis.drilldownCategory(x);
				} else {
					point.doDrilldown();
				}
			});
			/*wrap(point, 'importEvents', function (proceed) { // wrapping importEvents makes point.click event work
				if (!this.hasImportedEvents) {
					proceed.call(this);
					H.addEvent(this, 'click', function () {
						this.doDrilldown();
					});
				}
			});*/


			// Register drilldown points on this X value
			if (ddPointsX) {
				ddPointsX.push(point);
				ddPointsX.levelNumber = series.options._levelNumber;
			}

		}

		// Add or remove click handler and style on the tick label
		if (tick) {
			tick.drillable();
		}

		return point;
	});

	wrap(H.Series.prototype, 'drawDataLabels', function (proceed) {
		var css = this.chart.options.drilldown.activeDataLabelStyle;

		proceed.call(this);

		each(this.points, function (point) {
			if (point.drilldown && point.dataLabel) {
				point.dataLabel
					.attr({
						'class': 'highcharts-drilldown-data-label'
					})
					.css(css);
			}
		});
	});

	// Mark the trackers with a pointer 
	var type, 
		drawTrackerWrapper = function (proceed) {
			proceed.call(this);
			each(this.points, function (point) {
				if (point.drilldown && point.graphic) {
					point.graphic
						.attr({
							'class': 'highcharts-drilldown-point'
						})
						.css({ cursor: 'pointer' });
				}
			});
		};
	for (type in seriesTypes) {
		if (seriesTypes[type].prototype.supportsDrilldown) {
			wrap(seriesTypes[type].prototype, 'drawTracker', drawTrackerWrapper);
		}
	}
		
}));
