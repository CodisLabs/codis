
var CenteredSeriesMixin = Highcharts.CenteredSeriesMixin = {
	/**
	 * Get the center of the pie based on the size and center options relative to the
	 * plot area. Borrowed by the polar and gauge series types.
	 */
	getCenter: function () {

		var options = this.options,
			chart = this.chart,
			slicingRoom = 2 * (options.slicedOffset || 0),
			handleSlicingRoom,
			plotWidth = chart.plotWidth - 2 * slicingRoom,
			plotHeight = chart.plotHeight - 2 * slicingRoom,
			centerOption = options.center,
			positions = [pick(centerOption[0], '50%'), pick(centerOption[1], '50%'), options.size || '100%', options.innerSize || 0],
			smallestSize = mathMin(plotWidth, plotHeight),
			i,
			value;

		for (i = 0; i < 4; ++i) {
			value = positions[i];
			handleSlicingRoom = i < 2 || (i === 2 && /%$/.test(value));

			// i == 0: centerX, relative to width
			// i == 1: centerY, relative to height
			// i == 2: size, relative to smallestSize
			// i == 3: innerSize, relative to size
			positions[i] = relativeLength(value, [plotWidth, plotHeight, smallestSize, positions[2]][i]) +
				(handleSlicingRoom ? slicingRoom : 0);

		}
		// innerSize cannot be larger than size (#3632)
		if (positions[3] > positions[2]) {
			positions[3] = positions[2];
		}
		return positions;
	}
};

