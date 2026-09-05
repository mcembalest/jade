from std.sys import argv


def be32(data: List[UInt8], offset: Int) -> Int:
    return (
        (Int(data[offset]) << 24)
        | (Int(data[offset + 1]) << 16)
        | (Int(data[offset + 2]) << 8)
        | Int(data[offset + 3])
    )


def read(path: String) raises -> List[UInt8]:
    with open(path, "r") as file:
        return file.read_bytes()


def main() raises:
    var args = argv()
    if len(args) != 2:
        raise Error("usage: mojo mnist.mojo ..")
    var root = String(args[1])
    var train_images = read(root + "/data/train-images-idx3-ubyte")
    var train_labels = read(root + "/data/train-labels-idx1-ubyte")
    var test_images = read(root + "/data/t10k-images-idx3-ubyte")
    var test_labels = read(root + "/data/t10k-labels-idx1-ubyte")

    if be32(train_images, 0) != 2051 or be32(train_labels, 0) != 2049:
        raise Error("invalid MNIST training files")
    if be32(test_images, 0) != 2051 or be32(test_labels, 0) != 2049:
        raise Error("invalid MNIST test files")

    var pixels = be32(train_images, 8) * be32(train_images, 12)
    if pixels != be32(test_images, 8) * be32(test_images, 12):
        raise Error("training and test image shapes differ")

    var train_limit = be32(train_images, 4)
    if train_limit > 10000:
        train_limit = 10000
    var test_limit = be32(test_images, 4)
    if test_limit > 1000:
        test_limit = 1000

    var sums = List[Float64](length=10 * pixels, fill=0.0)
    var counts = List[Int](length=10, fill=0)
    for image in range(train_limit):
        var label = Int(train_labels[8 + image])
        counts[label] += 1
        for pixel in range(pixels):
            sums[label * pixels + pixel] += Float64(train_images[16 + image * pixels + pixel])

    for digit in range(10):
        for pixel in range(pixels):
            sums[digit * pixels + pixel] /= Float64(counts[digit])

    var correct = 0
    for image in range(test_limit):
        var best_digit = 0
        var best_distance = 1.0e300
        for digit in range(10):
            var distance = 0.0
            for pixel in range(pixels):
                var difference = (
                    Float64(test_images[16 + image * pixels + pixel])
                    - sums[digit * pixels + pixel]
                )
                distance += difference * difference
            if distance < best_distance:
                best_digit = digit
                best_distance = distance
        if best_digit == Int(test_labels[8 + image]):
            correct += 1

    var accuracy = Float64(correct) / Float64(test_limit)
    var report = String(
        t'{{\n  "implementation": "native-mojo",\n  "model": "nearest-centroid",\n  "training_images": {train_limit},\n  "test_images": {test_limit},\n  "correct": {correct},\n  "accuracy": {accuracy}\n}}\n'
    )
    with open("metrics.json", "w") as file:
        file.write(report)
    var table = String(
        t'# Mojo experiment — nearest centroid\n\n| training images | test images | correct | accuracy |\n|--:|--:|--:|--:|\n| {train_limit} | {test_limit} | {correct} | **{accuracy}** |\n\nMust match the Python baseline exactly.\n'
    )
    with open("report.md", "w") as file:
        file.write(table)
    print("accuracy:", accuracy)
