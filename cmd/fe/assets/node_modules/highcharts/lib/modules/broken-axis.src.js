/**
 * Highcharts JS v4.1.10 (2015-12-07)
 * Highcharts Broken Axis module
 * 
 * License: www.highcharts.com/license
 */

(function (factory) {
	if (typeof module === 'object' && module.exports) {
		module.exports = factory;
	} else {
		factory(Highcharts);
	}
}(function (H) {

	'use strict';

	var pick = H.pick,
		wrap = H.wrap,
		each = H.each,
		extend = H.extend,
		fireEvent = H.fireEvent,
		Axis = H.Axis,
		Series = H.Series;

	function stripArguments() {
		return Array.prototype.slice.call(arguments, 1);
	}

	extend(Axis.prototype, {
		isInBreak: function (brk, val) {
			var ret,
				repeat = brk.repeat || Infinity,
				from = brk.from,
				length = brk.to - brk.from,
				test = (val >= from ? (val - from) % repeat :  repeat - ((from - val) % repeat));

			if (!brk.inclusive) {
				ret = test < length && test !== 0;
			} else {
				ret = test <= length;
			}
			return ret;
		},

		isInAnyBreak: function (val, testKeep) {

			var breaks = this.options.breaks,
				i = breaks && breaks.length,
				inbrk,
				keep,
				ret;

			
			if (i) { 

				while (i--) {
					if (this.isInBreak(breaks[i], val)) {
						inbrk = true;
						if (!keep) {
							keep = pick(breaks[i].showPoints, this.isXAxis ? false : true);
						}
					}
				}

				if (inbrk && testKeep) {
					ret = inbrk && !keep;
				} else {
					ret = inbrk;
				}
			}
			return ret;
		}
	});

	wrap(Axis.prototype, 'setTickPositions', function (proceed) {
		proceed.apply(this, Array.prototype.slice.call(arguments, 1));
		
		if (this.options.breaks) {
			var axis = this,
				tickPositions = this.tickPositions,
				info = this.tickPositions.info,
				newPositions = [],
				i;

			for (i = 0; i < tickPositions.length; i++) {
				if (!axis.isInAnyBreak(tickPositions[i])) {
					newPositions.push(tickPositions[i]);
				}
			}

			this.tickPositions = newPositions;
			this.tickPositions.info = info;
		}
	});
	
	wrap(Axis.prototype, 'init', function (proceed, chart, userOptions) {
		// Force Axis to be not-ordinal when breaks are defined
		if (userOptions.breaks && userOptions.breaks.length) {
			userOptions.ordinal = false;
		}

		proceed.call(this, chart, userOptions);

		if (this.options.breaks) {

			var axis = this;
			
			axis.doPostTranslate = true;

			this.val2lin = function (val) {
				var nval = val,
					brk,
					i;

				for (i = 0; i < axis.breakArray.length; i++) {
					brk = axis.breakArray[i];
					if (brk.to <= val) {
						nval -= brk.len;
					} else if (brk.from >= val) {
						break;
					} else if (axis.isInBreak(brk, val)) {
						nval -= (val - brk.from);
						break;
					}
				}

				return nval;
			};
			
			this.lin2val = function (val) {
				var nval = val,
					brk,
					i;

				for (i = 0; i < axis.breakArray.length; i++) {
					brk = axis.breakArray[i];
					if (brk.from >= nval) {
						break;
					} else if (brk.to < nval) {
						nval += brk.len;
					} else if (axis.isInBreak(brk, nval)) {
						nval += brk.len;
					}
				}
				return nval;
			};

			this.setExtremes = function (newMin, newMax, redraw, animation, eventArguments) {
				// If trying to set extremes inside a break, extend it to before and after the break ( #3857 )
				while (this.isInAnyBreak(newMin)) {
					newMin -= this.closestPointRange;
				}				
				while (this.isInAnyBreak(newMax)) {
					newMax -= this.closestPointRange;
				}
				Axis.prototype.setExtremes.call(this, newMin, newMax, redraw, animation, eventArguments);
			};

			this.setAxisTranslation = function (saveOld) {
				Axis.prototype.setAxisTranslation.call(this, saveOld);

				var breaks = axis.options.breaks,
					breakArrayT = [],	// Temporary one
					breakArray = [],
					length = 0, 
					inBrk,
					repeat,
					brk,
					min = axis.userMin || axis.min,
					max = axis.userMax || axis.max,
					start,
					i,
					j;

				// Min & max check (#4247)
				for (i in breaks) {
					brk = breaks[i];
					repeat = brk.repeat || Infinity;
					if (axis.isInBreak(brk, min)) {
						min += (brk.to % repeat) - (min % repeat);
					}
					if (axis.isInBreak(brk, max)) {
						max -= (max % repeat) - (brk.from % repeat);
					}
				}

				// Construct an array holding all breaks in the axis
				for (i in breaks) {
					brk = breaks[i];
					start = brk.from;
					repeat = brk.repeat || Infinity;

					while (start - repeat > min) {
						start -= repeat;
					}
					while (start < min) {
						start += repeat;
					}

					for (j = start; j < max; j += repeat) {
						breakArrayT.push({
							value: j,
							move: 'in'
						});
						breakArrayT.push({
							value: j + (brk.to - brk.from),
							move: 'out',
							size: brk.breakSize
						});
					}
				}

				breakArrayT.sort(function (a, b) {
					var ret;
					if (a.value === b.value) {
						ret = (a.move === 'in' ? 0 : 1) - (b.move === 'in' ? 0 : 1);
					} else {
						ret = a.value - b.value;
					}
					return ret;
				});
				
				// Simplify the breaks
				inBrk = 0;
				start = min;

				for (i in breakArrayT) {
					brk = breakArrayT[i];
					inBrk += (brk.move === 'in' ? 1 : -1);

					if (inBrk === 1 && brk.move === 'in') {
						start = brk.value;
					}
					if (inBrk === 0) {
						breakArray.push({
							from: start,
							to: brk.value,
							len: brk.value - start - (brk.size || 0)
						});
						length += brk.value - start - (brk.size || 0);
					}
				}

				axis.breakArray = breakArray;

				fireEvent(axis, 'afterBreaks');
				
				axis.transA *= ((max - axis.min) / (max - min - length));

				axis.min = min;
				axis.max = max;
			};
		}
	});

	wrap(Series.prototype, 'generatePoints', function (proceed) {

		proceed.apply(this, stripArguments(arguments));

		var series = this,
			xAxis = series.xAxis,
			yAxis = series.yAxis,
			points = series.points,
			point,
			i = points.length,
			connectNulls = series.options.connectNulls,
			nullGap;


		if (xAxis && yAxis && (xAxis.options.breaks || yAxis.options.breaks)) {
			while (i--) {
				point = points[i];

				nullGap = point.y === null && connectNulls === false; // respect nulls inside the break (#4275)
				if (!nullGap && (xAxis.isInAnyBreak(point.x, true) || yAxis.isInAnyBreak(point.y, true))) {
					points.splice(i, 1);
					if (this.data[i]) {
						this.data[i].destroyElements(); // removes the graphics for this point if they exist
					}
				}
			}
		}

	});

	function drawPointsWrapped(proceed) {
		proceed.apply(this);
		this.drawBreaks();
	}

	H.Series.prototype.drawBreaks = function () {
		var series = this,
			points = series.points,
			axis,
			breaks,
			threshold,
			axisName = 'Axis',
			eventName,
			y;

		each(['y', 'x'], function (key) {
			axis = series[key + axisName];
			breaks = axis.breakArray || [];
			threshold = axis.isXAxis ? axis.min : pick(series.options.threshold, axis.min);
			each(points, function (point) {
				y = pick(point['stack' + key.toUpperCase()], point[key]);
				each(breaks, function (brk) {
					eventName = false;

					if ((threshold < brk.from && y > brk.to) || (threshold > brk.from && y < brk.from)) { 
						eventName = 'pointBreak';
					} else if ((threshold < brk.from && y > brk.from && y < brk.to) || (threshold > brk.from && y > brk.to && y < brk.from)) { // point falls inside the break
						eventName = 'pointInBreak'; // docs
					} 
					if (eventName) {
						fireEvent(axis, eventName, { point: point, brk: brk });
					}
				});
			});
		});
	};

	wrap(H.seriesTypes.column.prototype, 'drawPoints', drawPointsWrapped);
	wrap(H.Series.prototype, 'drawPoints', drawPointsWrapped);

}));
