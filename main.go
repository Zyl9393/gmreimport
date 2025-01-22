package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Zyl9393/gmreimport/yy"
)

type GMFrameReImport struct {
	Src        MasterImage
	Dst        GMSprite
	FrameIndex int
}

type MasterImage struct {
	SpriteName string
	FilePath   string
	Sha256     string
}

type GMSprite struct {
	Name   string
	Frames []GMFrame
}

type GMFrame struct {
	FileName       string
	Sha256         string
	Guid           string
	LayerFileName  string // we don't support multiple layers presently
	UtilizesLayers bool   // true if more than 1 layer, i.e. we're in the realm of the unsupported
}

func main() {
	log.SetFlags(0)
	params := parseArgs(os.Args[1:])
	candidatesSrcByName := findImportCandidates(params.SrcPath, make(map[string]MasterImage))
	candidatesDst := findGMSprites(params.DstPath, params)
	reImports := determineReImports(candidatesSrcByName, candidatesDst, params)
	numSpritesTouched := 0
	numFramesTouched := 0
	for _, reImport := range reImports {
		if reImport.FrameIndex == 0 {
			numSpritesTouched++
		}
		numFramesTouched++

		frame := reImport.Dst.Frames[reImport.FrameIndex]
		fromFilePath := reImport.Src.FilePath
		toFilePath1 := filepath.Join(params.DstPath, reImport.Dst.Name, frame.FileName)
		toFilePath2 := filepath.Join(params.DstPath, reImport.Dst.Name, "layers", frame.Guid, frame.LayerFileName)
		if params.IsDryRun {
			if !params.NoLogCopy {
				log.Printf("Would copy %#q over %#q and %#q.", fromFilePath, toFilePath1, toFilePath2)
			}
		} else {
			writeLog := !params.NoLogCopy
			copyFileL(fromFilePath, toFilePath1, writeLog)
			copyFileL(fromFilePath, toFilePath2, writeLog)
		}
	}
	if params.IsDryRun {
		log.Printf("Would have updated %d frames across %d sprites.", numFramesTouched, numSpritesTouched)
	} else {
		log.Printf("Updated %d frames across %d sprites.", numFramesTouched, numSpritesTouched)
	}
}

func copyFileL(fromFilePath, toFilePath string, writeLog bool) {
	copyFile(fromFilePath, toFilePath)
	if writeLog {
		log.Printf("Copied %#q to %#q.", fromFilePath, toFilePath)
	}
}

func copyFile(fromFilePath, toFilePath string) {
	fileFrom, err := os.Open(fromFilePath)
	if err != nil {
		log.Fatalf("Could not open %#q to copy from: %v", fromFilePath, err)
	}
	defer fileFrom.Close()
	fileTo, err := os.OpenFile(toFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Could not open %#q to copy over: %v", toFilePath, err)
	}
	defer fileTo.Close()
	_, err = io.Copy(fileTo, fileFrom)
	if err != nil {
		log.Fatalf("Could not copy %#q to %#q: %v", fromFilePath, toFilePath, err)
	}
}

func determineReImports(candidatesSrcByName map[string]MasterImage, candidatesDst []GMSprite, params *Parameters) (reImports []GMFrameReImport) {
	for _, spriteDst := range candidatesDst {
		if len(spriteDst.Frames) > 1 {
			const numDigitVariants = 4
			optionsFirstFrame := []string{spriteDst.Name + "_0000", spriteDst.Name + "_000", spriteDst.Name + "_00", spriteDst.Name + "_0",
				spriteDst.Name + "0000", spriteDst.Name + "000", spriteDst.Name + "00", spriteDst.Name + "0",
				spriteDst.Name + "_0001", spriteDst.Name + "_001", spriteDst.Name + "_01", spriteDst.Name + "_1",
				spriteDst.Name + "0001", spriteDst.Name + "001", spriteDst.Name + "01", spriteDst.Name + "1"}
			chosenOptionIndex := -1
			numDigits := 1
			useUnderscore := true
			isZeroBased := true
			for i, option := range optionsFirstFrame {
				if imageSrc, ok := candidatesSrcByName[option]; ok {
					if chosenOptionIndex >= 0 {
						if chosenOptionIndex < 2*numDigitVariants && i >= 2*numDigitVariants && chosenOptionIndex == i-2*numDigitVariants {
							continue
						}
						log.Fatalf("Multiple source image options for sprite %#q: %#q and %#q.", spriteDst.Name, candidatesSrcByName[optionsFirstFrame[chosenOptionIndex]].FilePath, imageSrc.FilePath)
					} else {
						numDigits = numDigitVariants - (i % numDigitVariants)
						useUnderscore = i/numDigitVariants%2 == 0
						isZeroBased = i < 2*numDigitVariants
						chosenOptionIndex = i
					}
				}
			}
			if chosenOptionIndex >= 0 {
				nameSep := "_"
				if !useUnderscore {
					nameSep = ""
				}
				bias := 0
				if !isZeroBased {
					bias = 1
				}
				for i := range spriteDst.Frames {
					suffix := fmt.Sprintf(fmt.Sprintf("%s%%0%dd", nameSep, numDigits), i+bias)
					srcImageName := spriteDst.Name + suffix
					imageSrc, ok := candidatesSrcByName[srcImageName]
					if ok {
						if imageSrc.Sha256 != spriteDst.Frames[i].Sha256 {
							reImports = append(reImports, GMFrameReImport{Src: imageSrc, Dst: spriteDst, FrameIndex: i})
						} else if !params.NoLogSrcMatch {
							log.Printf("Frame %d of sprite %#q already matches image %#q.", i, spriteDst.Name, imageSrc.FilePath)
						}
					} else if !params.NoLogSrcMiss {
						log.Printf("WARN: Found no image by file name %#q to reimport frame %d of sprite %#q.", srcImageName+".png", i, spriteDst.Name)
					}
				}
			} else if !params.NoLogSrcMiss {
				log.Printf("WARN: Found no source image by file name %#q or similar. Skipping other frames as well.", spriteDst.Name+"_0.png")
			}
		} else if len(spriteDst.Frames) == 1 {
			imageSrc, ok := candidatesSrcByName[spriteDst.Name]
			if ok {
				if imageSrc.Sha256 != spriteDst.Frames[0].Sha256 {
					reImports = append(reImports, GMFrameReImport{Src: imageSrc, Dst: spriteDst, FrameIndex: 0})
				} else if !params.NoLogSrcMatch {
					log.Printf("Sprite %#q already matches image %#q.", spriteDst.Name, imageSrc.FilePath)
				}
			} else if !params.NoLogSrcMiss {
				log.Printf("WARN: Found no source image by file name %#q.", spriteDst.Name+".png")
			}
		}
	}
	return reImports
}

func findImportCandidates(spritesPath string, candidatesByName map[string]MasterImage) map[string]MasterImage {
	spriteInfos, err := os.ReadDir(spritesPath)
	if err != nil {
		log.Fatalf("Read %#q: %v", spritesPath, err)
	}
	for _, spriteInfo := range spriteInfos {
		fileName := spriteInfo.Name()
		filePath := filepath.Join(spritesPath, fileName)
		if spriteInfo.IsDir() {
			candidatesByName = findImportCandidates(filePath, candidatesByName)
		} else {
			ext := strings.ToLower(filepath.Ext(fileName))
			if ext == ".png" {
				spriteName := fileName[:len(fileName)-len(ext)]
				existingSprite, alreadyInUse := candidatesByName[spriteName]
				if alreadyInUse {
					log.Fatalf("Duplicate file name: %#q and %#q.", filePath, existingSprite.FilePath)
				}
				sha256, err := sha256File(filePath)
				if err != nil {
					log.Fatalf("Get sha256 of %#q: %v", filePath, err)
				}
				candidatesByName[spriteName] = MasterImage{SpriteName: fileName, FilePath: filePath, Sha256: sha256}
			}
		}
	}
	return candidatesByName
}

func findGMSprites(spritesPath string, params *Parameters) (sprites []GMSprite) {
	spriteInfos, err := os.ReadDir(spritesPath)
	if err != nil {
		log.Fatalf("Read %#q: %v", spritesPath, err)
	}
	for _, spriteInfo := range spriteInfos {
		if spriteInfo.IsDir() {
			var sprite GMSprite
			sprite.Name = spriteInfo.Name()
			spritePath := filepath.Join(spritesPath, sprite.Name)
			yyPath := filepath.Join(spritePath, sprite.Name+".yy")
			spriteYY, err := yy.FromFile(yyPath)
			if err != nil {
				log.Fatalf("Read %#q: %v", yyPath, err)
			}
			sprite.Frames, err = parseYyFrames(spriteYY, spritePath)
			if err != nil {
				log.Fatalf("Interpret %#q: %v", yyPath, err)
			}
			canReImport := true
			for _, frame := range sprite.Frames {
				if frame.UtilizesLayers {
					canReImport = false
					if !params.NoLogDstBad {
						log.Printf("WARN: Not considering sprite %#q for re-import because it uses layers of GameMaker's built-in sprite editor, which this tool does not support.", sprite.Name)
					}
					break
				}
			}
			if canReImport {
				sprites = append(sprites, sprite)
			}
		} else {
			log.Fatalf("Encountered unexpected non-folder file %#q.", filepath.Join(spritesPath, spriteInfo.Name()))
		}
	}
	slices.SortFunc(sprites, func(a, b GMSprite) int {
		return strings.Compare(a.Name, b.Name)
	})
	return sprites
}

func sha256File(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sha256Encoder := sha256.New()
	_, err = io.Copy(sha256Encoder, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sha256Encoder.Sum(nil)), nil
}

func parseYyFrames(yy interface{}, spriteFolderPath string) (frames []GMFrame, err error) {
	frameData, ok := yy.(map[string]interface{})
	if frameData == nil || !ok {
		return nil, fmt.Errorf("root-element is not an object")
	}
	layers, ok := frameData["layers"].([]interface{})
	if layers == nil || !ok || len(layers) == 0 {
		return nil, fmt.Errorf("$.layers was not a non-empty array")
	}
	layer, ok := layers[0].(map[string]interface{})
	if layer == nil || !ok {
		return nil, fmt.Errorf("$.layers[0] was not an object")
	}
	layerName, ok := layer["name"].(string)
	if layerName == "" || !ok {
		return nil, fmt.Errorf("$.layers[0].name was not a non-empty string")
	}
	framesYy, ok := frameData["frames"].([]interface{})
	if framesYy == nil || !ok || len(framesYy) == 0 {
		return nil, fmt.Errorf("$.frames was not a non-empty array")
	}
	utilizesLayers := len(layers) > 1
	for i, frameInterface := range framesYy {
		frame, ok := frameInterface.(map[string]interface{})
		if frame == nil || !ok {
			return nil, fmt.Errorf("$.frames[%d] was not an object", i)
		}
		name, ok := frame["name"].(string)
		if name == "" || !ok {
			return nil, fmt.Errorf("$.frames[%d] was not a non-empty string", i)
		}
		var f GMFrame
		f.UtilizesLayers = utilizesLayers
		f.LayerFileName = layerName + ".png"
		f.Guid = name
		f.FileName = name + ".png"
		frameFilePath := filepath.Join(spriteFolderPath, f.FileName)
		f.Sha256, err = sha256File(frameFilePath)
		if err != nil {
			log.Fatalf("Get sha256 of %#q: %v", frameFilePath, err)
		}
		frames = append(frames, f)
	}
	return frames, nil
}

type Parameters struct {
	SrcPath       string
	DstPath       string
	IsDryRun      bool
	NoLogSrcMatch bool
	NoLogSrcMiss  bool
	NoLogDstBad   bool
	NoLogCopy     bool
}

func parseArgs(args []string) *Parameters {
	params := &Parameters{}
	fs := flag.NewFlagSet("main", flag.ContinueOnError)
	fs.StringVar(&params.SrcPath, "src", "", "Path to directory structure containing sprites")
	fs.StringVar(&params.DstPath, "dst", "", "Path to GameMaker project's sprites directory")
	fs.BoolVar(&params.IsDryRun, "dry", false, "If set, only log what the program would do instead of actually doing it")
	fs.BoolVar(&params.NoLogSrcMatch, "no-log-src-match", false, "If set, do not log about skipped re-imports caused by source image already matching destination sprite frame")
	fs.BoolVar(&params.NoLogSrcMiss, "no-log-src-miss", false, "If set, do not log about skipped re-imports caused by source image not being available for destination sprite frame")
	fs.BoolVar(&params.NoLogDstBad, "no-log-dst-bad", false, "If set, do not log about skipped re-imports caused by problems with destination sprite, such as it using multiple layers")
	fs.BoolVar(&params.NoLogCopy, "no-log-copy", false, "If set, do not log about re-import file copy operations")
	err := fs.Parse(args)
	if err != nil {
		log.Fatal(err)
	}
	if params.SrcPath == "" {
		log.Fatal("Must specify source path with -src.")
	}
	if params.DstPath == "" {
		log.Fatal("Must specify destination path with -dst.")
	}
	return params
}
