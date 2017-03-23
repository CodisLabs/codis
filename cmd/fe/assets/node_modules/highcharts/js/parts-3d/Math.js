/**
 *	Mathematical Functionility
 */
var PI = Math.PI,
	deg2rad = (PI / 180), // degrees to radians
	sin = Math.sin,
	cos = Math.cos,
	round = Math.round;

/**
 * Transforms a given array of points according to the angles in chart.options.
 * Parameters:
 *		- points: the array of points
 *		- chart: the chart
 *		- insidePlotArea: wether to verifiy the points are inside the plotArea
 * Returns:
 *		- an array of transformed points
 */
function perspective(points, chart, insidePlotArea) {
	var options3d = chart.options.chart.options3d,
		inverted = false,
		origin;

	if (insidePlotArea) {
		inverted = chart.inverted;
		origin = {
			x: chart.plotWidth / 2,
			y: chart.plotHeight / 2,
			z: options3d.depth / 2,
			vd: pick(options3d.depth, 1) * pick(options3d.viewDistance, 0)
		};
	} else {
		origin = {
			x: chart.plotLeft + (chart.plotWidth / 2),
			y: chart.plotTop + (chart.plotHeight / 2),
			z: options3d.depth / 2,
			vd: pick(options3d.depth, 1) * pick(options3d.viewDistance, 0)
		};
	}

	var result = [],
		xe = origin.x,
		ye = origin.y,
		ze = origin.z,
		vd = origin.vd,
		angle1 = deg2rad * (inverted ?  options3d.beta  : -options3d.beta),
		angle2 = deg2rad * (inverted ? -options3d.alpha :  options3d.alpha),
		s1 = sin(angle1),
		c1 = cos(angle1),
		s2 = sin(angle2),
		c2 = cos(angle2);

	var x, y, z, px, py, pz;

	// Transform each point
	each(points, function (point) {
		x = (inverted ? point.y : point.x) - xe;
		y = (inverted ? point.x : point.y) - ye;
		z = (point.z || 0) - ze;

		// Apply 3-D rotation
		// Euler Angles (XYZ): cosA = cos(Alfa|Roll), cosB = cos(Beta|Pitch), cosG = cos(Gamma|Yaw) 
		// 
		// Composite rotation:
		// |          cosB * cosG             |           cosB * sinG            |    -sinB    |
		// | sinA * sinB * cosG - cosA * sinG | sinA * sinB * sinG + cosA * cosG | sinA * cosB |
		// | cosA * sinB * cosG + sinA * sinG | cosA * sinB * sinG - sinA * cosG | cosA * cosB |
		// 
		// Now, Gamma/Yaw is not used (angle=0), so we assume cosG = 1 and sinG = 0, so we get:
		// |     cosB    |   0    |   - sinB    |
		// | sinA * sinB |  cosA  | sinA * cosB |
		// | cosA * sinB | - sinA | cosA * cosB |
		// 
		// But in browsers, y is reversed, so we get sinA => -sinA. The general result is:
		// |      cosB     |   0    |    - sinB     |     | x |     | px |
		// | - sinA * sinB |  cosA  | - sinA * cosB |  x  | y |  =  | py | 
		// |  cosA * sinB  |  sinA  |  cosA * cosB  |     | z |     | pz |
		//
		// Result: 
		px = c1 * x - s1 * z;
		py = -s1 * s2 * x + c2 * y - c1 * s2 * z;
		pz = s1 * c2 * x + s2 * y + c1 * c2 * z;


		// Apply perspective
		if ((vd > 0) && (vd < Number.POSITIVE_INFINITY)) {
			px = px * (vd / (pz + ze + vd));
			py = py * (vd / (pz + ze + vd));
		}

		//Apply translation
		px = px + xe;
		py = py + ye;
		pz = pz + ze;

		result.push({
			x: (inverted ? py : px),
			y: (inverted ? px : py),
			z: pz
		});
	});
	return result;
}
// Make function acessible to plugins
Highcharts.perspective = perspective;
